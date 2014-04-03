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
	"strconv"
)

type World struct {
	pkgs *[]string
	defs *[]string
	code *[]interface{}
	files *token.FileSet	
	exec string
	unstable bool
	write_src_mode bool
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

func exec_special(w *World, line string) bool {
	if line == "auto" {  // For autostarting
		*w.pkgs = append(*w.pkgs, "fmt")
		*w.pkgs = append(*w.pkgs, "math")
		*w.pkgs = append(*w.pkgs, "strings")
		*w.pkgs = append(*w.pkgs, "strconv")
		*w.defs = append(*w.defs, "var __Print_n, __Print_err = fmt.Print(\"\")")
		*w.defs = append(*w.defs, "var __Pi = math.Pi")
		*w.defs = append(*w.defs, "var __Trim_Nil = strings.Trim(\" \", \" \")")
		*w.defs = append(*w.defs, "var __Num_Itoa = strconv.Itoa(5)")
		w.unstable = compile(w).Len() > 0
		return true
	}
	if line == "run" {  // For running without a command
		if err := compile(w); err.Len() > 0 {
			fmt.Println("Compile error:", err)
		} else if out, err := run(); err.Len() > 0 {
			fmt.Println("Runtime error:\n", err)
		} else {
			fmt.Print(out)
		}
		return true
	}
	if line == "write" {  // For writing to source only
                w.write_src_mode = true
                return true
        }
        if line == "repl" {  // For running in repl mode (by default)
                w.write_src_mode = false
                return true
        }
	return false
}






// Code Removal
func cmd_remove_by_index(w *World, cmd_args string) {
	if len(cmd_args) == 0 {
		fmt.Println("Fatal error: cmd_args is empty")
		return
	}

	item_type := cmd_args[0]
	item_list_len := map[uint8]int{
		'd': len(*w.defs)+1,
		'p': len(*w.pkgs)+1,
		'c': len(*w.code)+1,
	}[item_type] - 1
	item_name := map[uint8]string {
		'd': "declarations",
		'p': "packages",
		'c': "code",
	}[item_type]

	if item_list_len == -1 {
		fmt.Printf("Remove: Invalid item type '%c'\n", item_type)
		return
	}
	if item_list_len == 0 {
		fmt.Printf("Remove: no more %s to remove\n", item_name)
		return
	}
	items_to_remove := cmd_remove_get_item_indices(item_list_len, cmd_args)

	switch item_type {
	case 'd':
		cmd_remove_declarations_by_index(w, items_to_remove)
	case 'p':
		cmd_remove_packages_by_index(w, items_to_remove)
	case 'c':
		cmd_remove_code_by_index(w, items_to_remove)
	default:
		fmt.Printf("Fatal error: Invalid item type '%c'\n", item_type)
		return
	}
}

func cmd_remove_get_item_indices(item_list_len int, cmd_args string) []bool {
	items_to_remove := make([]bool, item_list_len)

	if len(cmd_args) == 1 {
		items_to_remove[item_list_len - 1] = true
		return items_to_remove
	}

	item_indices := strings.Split(cmd_args[1:], ",")

	for _, item_index_str := range item_indices {
		if item_index_str == "" { continue }
		item_index, err := strconv.Atoi(item_index_str)
		if err != nil {
			fmt.Printf("Remove: %s not integer\n", item_index_str)
			continue
		}
		if item_index < 0 || item_index >= item_list_len {
			fmt.Printf("Remove: %d out of range\n", item_index)
			continue
		}
		if items_to_remove[item_index] {
			fmt.Printf("Remove: %d already in list\n", item_index)
			continue
		}
		items_to_remove[item_index] = true
	}

	return items_to_remove
}

// The unfortunate fact is that these three functions could not be merged, as
// the w.code type is different from the other two (and is necessary, since
// there is a need to keep track of if it is an assignment or not).
// Since Go is a static type language (and the interface{} type cannot be
// used here as intended), these three are left on their own. The rest has
// already been abstracted above, thankfully.
func cmd_remove_declarations_by_index(w *World, defs_to_remove []bool) {
	new_index := 0
	for old_index, def_item := range *w.defs {
		if defs_to_remove[old_index] {
			continue
		}
		(*w.defs)[new_index] = def_item
		new_index += 1
	}
	*w.defs = (*w.defs)[:new_index]
}

func cmd_remove_packages_by_index(w *World, pkgs_to_remove []bool) {
	new_index := 0
	for old_index, pkg_item := range *w.pkgs {
		if pkgs_to_remove[old_index] {
			continue
		}
		(*w.pkgs)[new_index] = pkg_item
		new_index += 1
	}
	*w.pkgs = (*w.pkgs)[:new_index]
}

func cmd_remove_code_by_index(w *World, code_to_remove []bool) {
	new_index := 0
	for old_index, code_item := range *w.code {
		if code_to_remove[old_index] {
			continue
		}
		(*w.code)[new_index] = code_item
		new_index += 1
	}
	*w.code = (*w.code)[:new_index]
}

func cmd_remove_packages_by_name(w *World, cmd_args string) {
	if len(cmd_args) == 0 {
		fmt.Println("Fatal error: cmd_args is empty")
		return
	}

	pkg_index_list := []string{}
	for _, pkg_name := range strings.Split(cmd_args, " ") {
		if pkg_name == "" { continue }
		pkg_index := -1
		for i, v := range *w.pkgs {
			if v == pkg_name {
				pkg_index = i
				break
			}
		}
		if pkg_index == -1 {
			fmt.Printf("Remove: No such package '%s'\n", pkg_name)
			continue
		}
		fmt.Printf("Remove: Removing '%s' at %d\n",pkg_name, pkg_index)
		pkg_index_list = append(pkg_index_list,strconv.Itoa(pkg_index))
	}
	cmd_remove_by_index(w, "p"+strings.Join(pkg_index_list, ","))
}











func main() {
	fmt.Println("Welcome to the Go REPL!")
	fmt.Println("Enter '?' for a list of commands.")

	w := new(World)
	w.pkgs = &[]string{}
	w.code = &[]interface{}{}
	w.defs = &[]string{}
	w.files = token.NewFileSet()
	w.unstable = false
	w.write_src_mode = false

	buf := bufio.NewReader(os.Stdin)
	for {
		if w.unstable {
			fmt.Print("! ")
		}

		fmt.Print(strings.Join(*w.pkgs, " ") + "> ")

		read, err := buf.ReadString('\n')
		if err != nil {
			println()
			break
		}

		line := strings.Trim(read[0 : len(read)-1], " ")
		if len(line) == 0 {
			continue
		}
		if strings.HasPrefix(line, "import") {
			line = strings.Replace(line, "import", "+", 1)
		}
		if exec_special(w, line) {
			continue
		}

		w.exec = ""

		switch line[0] {
		case '?':
			fmt.Println("Commands:")
			fmt.Println("\t?\thelp")
			fmt.Println("\timport (pkg) (pkg) ...\timport package")
			fmt.Println("\t+ (pkg) (pkg) ...\timport package")
			fmt.Println("\t- (pkg) (pkg) ...\tremove package")
			fmt.Println("\t-[dpc][#],[#],...\tpop last/specific (declaration|package|code)")
			fmt.Println("\t~\treset")
			fmt.Println("\t: (...)\tadd persistent code")
			fmt.Println("\t!\tinspect source")
			fmt.Println("\trun\trun source")
			fmt.Println("\twrite\twrite source mode on")
			fmt.Println("\trepl\twrite source mode off")
			fmt.Println("\tauto\tautomatically import standard package")
			fmt.Println("For removal, -[dpc] is equivalent to -[dpc]<last index>")
		case '+':
			allpkgs := strings.Split(strings.Trim(line[1:]," "), " ")
			fmt.Println("Importing: ")
			for _, pkg_name := range allpkgs {
				if pkg_name != "" {
					*w.pkgs = append(*w.pkgs, pkg_name)
					fmt.Println(" ", len(*w.pkgs), pkg_name)
				}
			}
			w.unstable = compile(w).Len() > 0
		case '-':
			if len(cmd_args) == 0 {
				fmt.Println("No item specified for removal.")
			} else if line[1] != ' ' {
				cmd_remove_by_index(w, cmd_args)
			} else {
				cmd_remove_packages_by_name(w, cmd_args)
			}
			w.unstable = compile(w).Len() > 0
		case '~':
			*w.pkgs = (*w.pkgs)[:0]
			*w.defs = (*w.defs)[:0]
			*w.code = (*w.code)[:0]
			w.unstable = false
			w.write_src_mode = false
		case '!':
			fmt.Println(w.source_print(true))
		case ':':
			line = line + ";"
			tree, err := ParseStmtList(w.files, "go-repl", strings.Trim(line[1:]," "))
			if err != nil {
				fmt.Println("Parse error:", err)
				continue
			}

			for _, v := range tree {
				*w.code = append(*w.code, v)
			}

			w.unstable = compile(w).Len() > 0
		default:
			line = line + ";"
			var tree interface{}
			tree, err := ParseDeclList(w.files, "go-repl", line[0:])
			if err != nil {
				tree, err = ParseStmtList(w.files, "go-repl", line[0:])
				if err != nil {
					fmt.Println("Parse error:", err)
					continue
				} else {
					fmt.Println("CODE: ", line[0:])
				}
			} else {
				fmt.Println("DECL: ", line[0:])
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
					if w.write_src_mode {
						*w.code = append(*w.code, v)
						continue
					}
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
			default:
				fmt.Println("Fatal error: Unknown tree type.")
			}

			if w.write_src_mode { continue }

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
				w.unstable = true
				fmt.Println("Fatal error: Code should not run")
			}

			if changed {
				w.unstable = compile(w).Len() > 0
			}
		}
	}
}
