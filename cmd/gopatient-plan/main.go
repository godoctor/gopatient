package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	//"go/build"
	"go/parser"
	"go/token"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	//"code.google.com/p/go.tools/go/loader"
)

func usage() {
	fmt.Fprintln(os.Stderr, `Usage: gopatient-plan -pkglist file -find {files|idents} [-limit n] [-seed n] [-csv file] -template file
Creates a test plan, saving a lists of tests to a CSV (comma-separated values)
file (default: tests.csv), as well as a Makefile to run the tests

The template file will be substituted into the body of each rule in the
Makefile.  The following can be used in the template:
	!NUM!     will be replaced with the test number (1, 2, 3, ...)
	!PKG!     will be replaced with the package name, like "mypackage/sub"
	!POS!     will be replaced with a position string, like "1,3:1,3"
	!FILE!    will be replaced with the absolute path to the file

Arguments:`)
	flag.PrintDefaults()
	os.Exit(2)
}

func init() {
	flag.Usage = usage
}

var (
	pkgsFlag = flag.String("pkglist", "",
		"Text file containing a list of go packages, one per line")
	findFlag = flag.String("find", "idents",
		"What to find: files or idents (default: files)")
	limitFlag = flag.Int("limit", 50,
		"Number of tests to generate (default: 50)")
	seedFlag = flag.Int64("seed", 0,
		"Seed for random number generator (default: 0)")
	csvFlag = flag.String("csv", "",
		"Filename to write CSV list of tests (default: omit)")
	templateFlag = flag.String("template", "",
		"Text file containing a template for Makefile targets")
)

const (
	numMacro  = "!NUM!"
	pkgMacro  = "!PKG!"
	posMacro  = "!POS!"
	fileMacro = "!FILE!"
)

func main() {
	flag.Parse()
	if flag.NFlag() == 0 || flag.NArg() > 0 {
		usage()
	}

	if *templateFlag == "" {
		fmt.Fprintln(os.Stderr, "-template is required")
		os.Exit(1)
	}
	template, err := readLines(*templateFlag)
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}

	if *pkgsFlag == "" {
		fmt.Fprintln(os.Stderr, "-pkglist is required")
		os.Exit(1)
	}
	pkgNames, err := readLines(*pkgsFlag)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}

	finder, err := finder(*findFlag)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}

	rnd := rand.New(rand.NewSource(*seedFlag))
	tests := permuteNumberAndLimit(createTests(pkgNames, finder), rnd)

	err = writeCSV(tests)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}

	err = writeMakefile(tests, template)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}

func readLines(filename string) ([]string, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return []string{}, err
	}
	lines := strings.Split(string(bytes), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return lines, nil
}

func finder(typ string) (func(*token.FileSet, *ast.File) []string, error) {
	switch typ {
	case "files":
		return fileFinder, nil
	case "idents":
		return identFinder, nil
	default:
		return func(*token.FileSet, *ast.File) []string {
				return []string{}
			},
			fmt.Errorf("Invalid -find flag: %s", typ)
	}
}

func createTests(pkgNames []string, finder func(*token.FileSet, *ast.File) []string) []string {
	tests := []string{}
	for _, pkgName := range pkgNames {
		fpath := filepath.Join(os.Getenv("GOPATH"), "src", pkgName)
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, fpath, nil, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing %s", err)
			os.Exit(1)
		}

		for _, pkg := range pkgs {
			expectedPkgName := path.Base(pkgName)
			actualPkgName := pkg.Name
			if expectedPkgName != actualPkgName {
				continue
			}
			for fname, file := range pkg.Files {
				for _, rgn := range finder(fset, file) {
					tests = append(tests,
						fmt.Sprintf("%s\t%s\t\"%s\"",
							pkgName, fname, rgn))
				}
			}
		}
	}
	sort.Strings(tests) // To ensure determinism for a given random seed
	return tests
}

//func createTests(pkgNames []string, finder func(*token.FileSet, *ast.File) []string) []string {
//	tests := []string{}
//	for _, pkgName := range pkgNames {
//		var lconfig loader.Config
//		lconfig.Build = &build.Default
//		lconfig.ParserMode = parser.DeclarationErrors
//		lconfig.AllowErrors = false
//		lconfig.SourceImports = true
//		lconfig.FromArgs([]string{ pkgName }, true)
//		p, err := lconfig.Load()
//		if err != nil {
//			fmt.Fprintf(os.Stderr, "Error parsing %s", err)
//			os.Exit(1)
//		}
//
//		for _, pkgInfo := range p.InitialPackages() {
//			expectedPkgName := path.Base(pkgName)
//			actualPkgName := pkgInfo.Pkg.Name()
//			if expectedPkgName != actualPkgName {
//				continue
//			}
//			for _, file := range pkgInfo.Files {
//				fname := p.FilePath(file)
//				for _, rgn := range finder(p.Fset, file) {
//					tests = append(tests,
//						fmt.Sprintf("%s\t%s\t\"%s\"",
//							pkgName, fname, rgn))
//				}
//			}
//		}
//	}
//	sort.Strings(tests) // To ensure determinism for a given random seed
//	return tests
//}

func permuteNumberAndLimit(tests []string, rnd *rand.Rand) []string {
	permuted := make([]string, len(tests))
	for idx, newIdx := range rnd.Perm(len(tests)) {
		permuted[newIdx] = tests[idx]
	}
	if len(permuted) > *limitFlag {
		permuted = permuted[0:*limitFlag]
	}
	for idx := 0; idx < len(permuted); idx++ {
		permuted[idx] = fmt.Sprintf("%d\t%s", idx+1, permuted[idx])
	}
	return permuted
}

func writeCSV(tests []string) error {
	if *csvFlag == "" {
		return nil
	}

	file, err := os.OpenFile(*csvFlag,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		0666)
	if err != nil {
		return err
	}
	out := bufio.NewWriter(file)

	for _, t := range tests {
		fmt.Fprintln(out, strings.Replace(t, "\t", ",", -1))
	}

	err = out.Flush()
	if err != nil {
		return err
	}

	err = file.Close()
	if err != nil {
		return err
	}
	return nil
}

func writeMakefile(tests []string, template []string) error {
	file, err := os.OpenFile("Makefile",
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		0666)
	if err != nil {
		return err
	}
	out := bufio.NewWriter(file)

	fmt.Fprintf(out, "all: init ")
	for _, t := range tests {
		fields := strings.Split(t, "\t")
		num := fields[0]
		fmt.Fprintf(out, " %s.success", num)
	}
	fmt.Fprintln(out, "\ninit:\n\t@if [ `pwd` != $$GOPATH ]; then echo \"This Makefile must be run from the root of the GOPATH ($$GOPATH)\"; exit 1; fi")
	fmt.Fprintf(out, `
prepare: .patient-backup/ok

.patient-backup/ok:
	@if [ -d .patient-backup ]; then echo ".patient-backup already exists, but .patient-backup/ok does not.  Incomplete backup?  Try rm -rf .patient-backup and run again"; exit 1; fi
	mkdir .patient-backup
	mkdir .patient-backup/bin
	mkdir .patient-backup/pkg
	rsync -av src .patient-backup/
	touch .patient-backup/ok
`)

	for _, t := range tests {
		fields := strings.Split(t, "\t")
		num := fields[0]
		pkg := fields[1]
		fname := fields[2]
		pos := fields[3]

		fmt.Fprintf(out, "\n%s.success: .patient-backup/ok\n", num)
		fmt.Fprintf(out, `
	@if [ ! -d .patient-backup/src ]; then echo ".patient-backup/src does not exist; cannot restore"; exit 1; fi
	rsync -av --delete .patient-backup/{bin,pkg,src} .
`)

		for _, line := range template {
			if line == "" {
				continue
			}
			line = strings.Replace(line, numMacro, num, -1)
			line = strings.Replace(line, pkgMacro, pkg, -1)
			line = strings.Replace(line, fileMacro, fname, -1)
			line = strings.Replace(line, posMacro, pos, -1)
			fmt.Fprintf(out, "\t%s\n", line)
		}
		fmt.Fprintf(out, "\ttouch %s.success\n", num)
	}

	fmt.Fprintf(out, "\nclean:\n")
	for _, t := range tests {
		fields := strings.Split(t, "\t")
		num := fields[0]
		fmt.Fprintf(out, "\trm -f %s.success\n", num)
	}

	err = out.Flush()
	if err != nil {
		return err
	}

	err = file.Close()
	if err != nil {
		return err
	}
	return nil
}

func fileFinder(fset *token.FileSet, file *ast.File) []string {
	return []string{"1,1:1,1"}
}

func identFinder(fset *token.FileSet, file *ast.File) []string {
	result := []string{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch id := n.(type) {
		case *ast.Ident:
			fromLine := fset.Position(id.Pos()).Line
			fromCol := fset.Position(id.Pos()).Column
			toLine := fset.Position(id.End()).Line
			toCol := fset.Position(id.End()).Column
			result = append(result, fmt.Sprintf("%d,%d:%d,%d",
				fromLine, fromCol, toLine, toCol))
		}
		return true
	})
	return result
}
