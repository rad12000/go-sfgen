package main

import (
	"fmt"
	"go/types"
	"golang.org/x/tools/go/packages"
	"log"
	"maps"
	"os"
	"slices"
	"sync"
)

var packageNameToScopes = make(map[string]*packages.Package)

type packageToLoad struct {
	Dir          string
	PackageName  string
	IncludeTests bool
}

func (p packageToLoad) String() string {
	return p.Dir
}

func (p packageToLoad) Key() string {
	return fmt.Sprintf("%s%s%v", p.Dir, p.PackageName, p.IncludeTests)
}

// loadPackageScopes loads concurrently loads all package scopes for the provided package names one time.
// Note: this function should be called once, and is not thread safe.
func loadPackageScopes(packagesToLoad []packageToLoad) {
	var (
		seenPackages = make(map[string]struct{})
		errCh        = make(chan error)
		doneCh       = make(chan struct{})
		wg           sync.WaitGroup
	)

	for _, p := range packagesToLoad {
		if _, ok := seenPackages[p.Key()]; ok {
			continue
		}

		seenPackages[p.Key()] = struct{}{}
		packageNameToScopes[p.Key()] = nil // this avoids having to lock by taking the place in the map immediately
		wg.Add(1)
		go func(p *packageToLoad) {
			defer wg.Done()
			cfg := packages.Config{
				Mode:  packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
				Tests: p.IncludeTests,
				//Dir:   p.Dir,
			}

			var patterns []string
			if p.PackageName != "" {
				patterns = append(patterns, p.PackageName)
			}

			loadedPkg, err := packages.Load(&cfg, p.Dir)
			if err != nil {
				errCh <- fmt.Errorf("failed to load package %s: %w", p, err)
				return
			}

			packagesPathsToPkg := make(map[string]*packages.Package)
			for _, p := range loadedPkg {
				key := fmt.Sprintf("%s/%s", p.Dir, p.Name)
				if currentP, ok := packagesPathsToPkg[key]; ok {
					if len(p.ID) > len(currentP.ID) {
						continue
					}
				}
				packagesPathsToPkg[key] = p
			}

			pkgs := slices.Collect(maps.Values(packagesPathsToPkg))
			if len(pkgs) != 1 && p.PackageName != "" {
				filteredPkgs := pkgs[:0]
				for _, p2 := range pkgs {
					if p2.Name == p.PackageName {
						filteredPkgs = append(filteredPkgs, p2)
					}
				}

				pkgs = filteredPkgs
			}

			if len(pkgs) != 1 {
				for _, p := range packagesPathsToPkg {
					fmt.Fprintf(os.Stderr, "Found package %s#%s\n", p.Dir, p.Name)
				}
				errCh <- fmt.Errorf("failed to load package %s: expected to find 1 package, found %d", p, len(pkgs))
				return
			}

			if len(pkgs[0].Errors) > 0 {
				errCh <- fmt.Errorf("failed to load package %s: %v", p, loadedPkg[0].Errors)
				return
			}

			scope := pkgs[0].Types.Scope()
			if scope == nil {
				errCh <- fmt.Errorf("failed to load package %s: could not load scope", p)
				return
			}

			packageNameToScopes[p.Key()] = pkgs[0]
		}(&p)
	}

	go func() {
		wg.Wait()
		close(doneCh)
	}()

	for {
		select {
		case err := <-errCh:
			log.Fatal(err)
		case <-doneCh:
			return
		}
	}
}

// scopeForPackage should only be called after loadPackageScopes has been
func scopeForPackage(packageToLoad packageToLoad) (*packages.Package, *types.Scope, bool) {
	p, ok := packageNameToScopes[packageToLoad.Key()]
	return p, p.Types.Scope(), ok
}
