package main

import (
	"fmt"
	"go/types"
	"golang.org/x/tools/go/packages"
	"log"
	"sync"
)

var packageNameToScopes = make(map[string]*types.Scope)

// loadPackageScopes loads concurrently loads all package scopes for the provided package names one time.
// Note: this function should be called once, and is not thread safe.
func loadPackageScopes(packageDirs []string) {
	var (
		seenPackages = make(map[string]struct{})
		errCh        = make(chan error)
		doneCh       = make(chan struct{})
		wg           sync.WaitGroup
	)

	for _, p := range packageDirs {
		if _, ok := seenPackages[p]; ok {
			continue
		}

		seenPackages[p] = struct{}{}
		packageNameToScopes[p] = nil // this avoids having to lock by taking the place in the map immediately
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			cfg := packages.Config{
				Mode: packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
			}

			loadedPkg, err := packages.Load(&cfg, p)
			if err != nil {
				errCh <- fmt.Errorf("failed to load package %s: %w", p, err)
				return
			}

			if len(loadedPkg) != 1 {
				errCh <- fmt.Errorf("failed to load package %s: expected to find 1 package, found %d", p, len(p))
				return
			}

			if len(loadedPkg[0].Errors) > 0 {
				errCh <- fmt.Errorf("failed to load package %s: %v", p, loadedPkg[0].Errors)
				return
			}

			scope := loadedPkg[0].Types.Scope()
			if scope == nil {
				errCh <- fmt.Errorf("failed to load package %s: could not load scope", p)
				return
			}

			packageNameToScopes[p] = scope
		}(p)
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
func scopeForPackage(packageName string) (*types.Scope, bool) {
	p, ok := packageNameToScopes[packageName]
	return p, ok
}
