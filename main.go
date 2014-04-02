package main

import (
	"os/exec"
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/printer"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

type World struct {
	pkgs *[]string
	defs *[]string
	code *[]interface{}
	files *token.FileSet	
	exec string
}

const TEMPPATH = "/tmp/gorepl"


var (
	bin  = os.Getenv("GOBIN")
	arch = map[string]string{
		"amd64": "6",
		"386":   "8",
		"arm":   "5",
	}[os.Getenv("GOARCH")]
)

func indentCode(text string, indent string) string {
	return strings.Join(strings.Split(text, "\n"), "\n"+indent)
}

func (self *World) source() string {
	return self.source_print(false)
}

func (self *World) source_print(print_linenums bool) string {
	source := "package main\n"

	pkgs_num := 0
	defs_num := 0
	code_num := 0
	if print_linenums { source = "\n    " + source }

	for _, v := range *self.pkgs {
		if print_linenums {
			source += "p"+strconv.Itoa(pkgs_num)+": "
			pkgs_num += 1
		}
		source += "import \"" + v + "\"\n"
	}

	source += "\n"

	for _, d := range *self.defs {
		if print_linenums {
			source += "d"+strconv.Itoa(defs_num)+": "
			defs_num += 1
		}
		source += indentCode(d, "    ") + "\n\n"
	}

	if print_linenums { source += "    " }
	source += "func noop(_ interface{}) {}\n\n"
	if print_linenums { source += "    " }
	source += "func main() {\n"

	for _, c := range *self.code {
		str := new(bytes.Buffer)
		printer.Fprint(str, self.files, c)

		if print_linenums {
			source += "c"+strconv.Itoa(code_num)+": "
			code_num += 1
		}
		source += "\t" + indentCode(str.String(), "\t") + ";\n"
		switch c.(type) {
		case *ast.AssignStmt:
			for _, name := range c.(*ast.AssignStmt).Lhs {
				str := new(bytes.Buffer)
				printer.Fprint(str, self.files, name)
				source += "\t" + "noop(" + str.String() + ");\n"
			}
		}
	}

	if self.exec != "" {
		source += "\t" + self.exec + ";\n"
	}

	if print_linenums { source += "    " }
	source += "}\n"

	return source
}

func compile(w *World) *bytes.Buffer {
	ioutil.WriteFile(TEMPPATH+".go", []byte(w.source()), 0644)

	errBuf := new(bytes.Buffer)

	if arch == "" {
		arch = "6"   // Most likely 64-bit architecture
	}
	cmd := exec.Command("echo", "")

	if bin != "" {
		cmd = exec.Command(bin+"/"+arch+"g",
			"-o", TEMPPATH + "."+arch, TEMPPATH + ".go")
	} else {
		cmd = exec.Command("go", "tool", arch+"g",
			"-o", TEMPPATH + "."+arch, TEMPPATH + ".go")
	}
	cmdout,err := cmd.StdoutPipe()
	err = cmd.Start()
	if err != nil {
		panic(err)
	}
	io.Copy(errBuf, cmdout)
	err = cmd.Wait()
	if errBuf.Len() > 0 {
		return errBuf
	}

	if bin != "" {
		cmd = exec.Command(bin+"/"+arch+"l",
			"-o", TEMPPATH, TEMPPATH + "."+arch)
	} else {
		cmd = exec.Command("go", "tool", arch+"l",
			"-o", TEMPPATH, TEMPPATH + "."+arch)
	}
	cmdout,err = cmd.StdoutPipe()

	err = cmd.Start()
	if err != nil {
		panic(err)
	}
	io.Copy(errBuf, cmdout)
	err = cmd.Wait()

	return errBuf
}

func run() (*bytes.Buffer, *bytes.Buffer) {
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)

	cmd := exec.Command(
		TEMPPATH,TEMPPATH)

	stdout,err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	stderr,err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}
	err = cmd.Start()
	if err != nil {
		panic(err)
	}
	io.Copy(errBuf, stderr)

	if errBuf.Len() > 0 {
		return nil, errBuf
	}

	io.Copy(outBuf, stdout)

	return outBuf, errBuf
}

func intf2str(src interface{}) string {
	switch s := src.(type) {
		case string:
			return s
	}
	return ""
}

func ParseStmtList(fset *token.FileSet, filename string, src interface{}) ([]ast.Stmt, error) {
	//f, err := parser.ParseFile(fset, filename, src, 0)
	f, err := parser.ParseFile(fset, filename, "package p;func _(){"+intf2str(src)+"\n}", 0)
	if err != nil {
		return nil, err
	}
	return f.Decls[0].(*ast.FuncDecl).Body.List, nil
}

func ParseDeclList(fset *token.FileSet, filename string, src interface{}) ([]ast.Decl, error) {
	//f, err := parser.ParseFile(fset, filename, src, 0)
	f, err := parser.ParseFile(fset, filename, "package p;"+intf2str(src), 0)
	if err != nil {
		return nil, err
	}
	return f.Decls, nil
}

func main() {
	fmt.Println("Welcome to the Go REPL!")
	fmt.Println("Enter '?' for a list of commands.")

	w := new(World)
	w.pkgs = &[]string{}
	w.code = &[]interface{}{}
	w.defs = &[]string{}
	w.files = token.NewFileSet()

	buf := bufio.NewReader(os.Stdin)
	unstable := false
	for {
		if unstable {
			fmt.Print("! ")
		}

		fmt.Print(strings.Join(*w.pkgs, " ") + "> ")

		read, err := buf.ReadString('\n')
		if err != nil {
			println()
			break
		}

		line := read[0 : len(read)-1]
		if len(line) == 0 {
			continue
		}

		w.exec = ""

		switch line[0] {
		case '?':
			fmt.Println("Commands:")
			fmt.Println("\t?\thelp")
			fmt.Println("\t+ (pkg)\timport package")
			fmt.Println("\t- (pkg)\tremove package")
			fmt.Println("\t-[dpc]\tpop last (declaration|package|code)")
			fmt.Println("\t~\treset")
			fmt.Println("\t: (...)\tadd persistent code")
			fmt.Println("\t!\tinspect source")
		case '+':
			allpkgs := strings.Split(strings.Trim(line[1:]," "), " ")
			fmt.Println("Importing: ")
			for _, pkg_name := range allpkgs {
				if pkg_name != "" {
					*w.pkgs = append(*w.pkgs, pkg_name)
					fmt.Println(" ", len(*w.pkgs), pkg_name)
				}
			}
			unstable = compile(w).Len() > 0
		case '-':
			if len(line) > 1 && line[1] != ' ' {
				switch line[1] {
				case 'd':
					if len(*w.defs) > 0 {
						*w.defs = (*w.defs)[:len(*w.defs)-1]
					}
				case 'p':
					if len(*w.pkgs) > 0 {
						*w.pkgs = (*w.pkgs)[:len(*w.pkgs)-1]
					}
				case 'c':
					if len(*w.code) > 0 {
						*w.code = (*w.code)[:len(*w.code)-1]
					}
				}
			} else {
				if len(line) > 2 && len(*w.pkgs) > 0 {
					for i, v := range *w.pkgs {
						if v == line[2:] {
							copy((*w.pkgs)[i:], (*w.pkgs)[i+1:])
							*w.pkgs = (*w.pkgs)[:len(*w.pkgs)-1]
							break
						}
					}
				} else {
					if len(*w.code) > 0 {
						*w.code = (*w.code)[:len(*w.code)-1]
					}
				}
			}

			unstable = compile(w).Len() > 0
		case '~':
			*w.pkgs = (*w.pkgs)[:0]
			*w.defs = (*w.pkgs)[:0]
			*w.code = (*w.code)[:0]
			unstable = false
		case '!':
			fmt.Println(w.source_print(true))
		case ':':
			line = line + ";"
			tree, err := ParseStmtList(w.files, "go-repl", strings.Trim(line[1:]," "))
			if err != nil {
				fmt.Println("Parse error:", err)
				continue
			}

			*w.code = append(*w.code,tree[0])

			unstable = compile(w).Len() > 0
		default:
			line = line + ";"
			var tree interface{}
			tree, err := ParseStmtList(w.files, "go-repl", line[0:])
			if err != nil {
				tree, err = ParseDeclList(w.files, "go-repl", line[0:])
				if err != nil {
					fmt.Println("Parse error:", err)
					continue
				}
			}

			changed := false
			got_err := false
			bkup_pkgs := *w.pkgs
			bkup_code := *w.code
			bkup_defs := *w.defs
			bkup_files := w.files
			bkup_exec := w.exec

			switch tree.(type) {
			case []ast.Stmt:
				for _, v := range tree.([]ast.Stmt) {
					str := new(bytes.Buffer)
					printer.Fprint(str, w.files, v)

					switch v.(type) {
					case *ast.AssignStmt:
						*w.code = append(*w.code,v)
						changed = true
					default:
						w.exec = str.String()
					}
				}
			case []ast.Decl:
				for _, v := range tree.([]ast.Decl) {
					str := new(bytes.Buffer)
					printer.Fprint(str, w.files, v)

					*w.defs = append(*w.defs,str.String())
				}

				changed = true
			}

			if err := compile(w); err.Len() > 0 {
				fmt.Println("Compile error:", err)
				got_err = true
			} else if out, err := run(); err.Len() > 0 {
				fmt.Println("Runtime error:\n", err)
				got_err = true
			} else {
				fmt.Print(out)
			}
			
			if got_err {
				*w.pkgs = bkup_pkgs
				*w.code = bkup_code
				*w.defs = bkup_defs
				w.files = bkup_files
				w.exec = bkup_exec
				continue
			}

			if changed && got_err {
				unstable = true
				fmt.Println("Fatal error: Code should not run")
			}

			if changed {
				unstable = compile(w).Len() > 0
			}
		}
	}
}
