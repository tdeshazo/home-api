package app

import (
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"
)

var (
	htmlAssetRefRE = regexp.MustCompile(`\b(?:href|src)="(/assets/[^"#?]+)`)
	jsImportRE     = regexp.MustCompile(`(?s)\b(?:import|export)\s+.*?\sfrom\s+["'](\./[^"']+)["']|\bimport\s*\(\s*["'](\./[^"']+)["']`)
	cssURLRE       = regexp.MustCompile(`url\(([^)]+)\)`)
)

func TestEmbeddedFrontendReferences(t *testing.T) {
	htmlFiles, err := fs.Glob(Files, "*.html")
	if err != nil {
		t.Fatalf("glob html files: %v", err)
	}
	if len(htmlFiles) == 0 {
		t.Fatal("no embedded HTML files found")
	}

	for _, file := range htmlFiles {
		t.Run(file, func(t *testing.T) {
			body, err := fs.ReadFile(Files, file)
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			for _, match := range htmlAssetRefRE.FindAllStringSubmatch(string(body), -1) {
				ref := strings.TrimPrefix(match[1], "/")
				if _, err := fs.Stat(Files, ref); err != nil {
					t.Fatalf("%s references missing asset %q: %v", file, match[1], err)
				}
			}
		})
	}
}

func TestEmbeddedJavaScriptImports(t *testing.T) {
	jsFiles, err := fs.Glob(Files, "assets/*.js")
	if err != nil {
		t.Fatalf("glob JavaScript files: %v", err)
	}
	if len(jsFiles) == 0 {
		t.Fatal("no embedded JavaScript files found")
	}

	for _, file := range jsFiles {
		t.Run(file, func(t *testing.T) {
			body, err := fs.ReadFile(Files, file)
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			for _, match := range jsImportRE.FindAllStringSubmatch(string(body), -1) {
				specifier := firstNonEmpty(match[1], match[2])
				ref := path.Clean(path.Join(path.Dir(file), specifier))
				if !strings.HasPrefix(ref, "assets/") {
					continue
				}
				if _, err := fs.Stat(Files, ref); err != nil {
					t.Fatalf("%s imports missing module %q: %v", file, specifier, err)
				}
			}
		})
	}
}

func TestEmbeddedCSSAssetURLs(t *testing.T) {
	cssFiles, err := fs.Glob(Files, "assets/*.css")
	if err != nil {
		t.Fatalf("glob CSS files: %v", err)
	}
	if len(cssFiles) == 0 {
		t.Fatal("no embedded CSS files found")
	}

	for _, file := range cssFiles {
		t.Run(file, func(t *testing.T) {
			body, err := fs.ReadFile(Files, file)
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			for _, match := range cssURLRE.FindAllStringSubmatch(string(body), -1) {
				specifier := strings.Trim(strings.TrimSpace(match[1]), `"'`)
				if specifier == "" || strings.HasPrefix(specifier, "data:") || strings.Contains(specifier, "://") {
					continue
				}

				ref := strings.TrimPrefix(specifier, "/")
				if !strings.HasPrefix(ref, "assets/") {
					ref = path.Clean(path.Join(path.Dir(file), specifier))
				}
				if _, err := fs.Stat(Files, ref); err != nil {
					t.Fatalf("%s references missing URL asset %q: %v", file, specifier, err)
				}
			}
		})
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
