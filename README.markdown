A compiling Go REPL.

Builds up Go source as the session goes on, compiles and runs it with every input.

A "!" in front of the input means it's in "unstable" mode, e.g. a package has been imported and isn't used, or errors occur in the source.

Example session:

    Welcome to the Go REPL!
    Enter '?' for a list of commands.
    > ?
    Commands:
        ?	help
        + (pkg)	import package
        - (pkg)	remove package
        -[dpc]	pop last (declaration|package|code)
        ~	reset
        : (...)	add persistent code
        !	inspect source
    > a := 6
    > b := 7
    > println(a * b)
    42
    > + fmt
    ! fmt> fmt.Println("Hello, world!")
    Hello, world!
    ! fmt> println("This won't work since fmt doesn't get used.")
    Compile error: /tmp/gorepl.go:2: imported and not used: fmt

    ! fmt> : fmt.Print()
    fmt> println("Now it will!")
    Now it will!
    fmt> func b(a interface{}) { fmt.Printf("You passed: %#v\n", a); }
    fmt> b(1)
    Compile error: /tmp/gorepl.go:14: cannot call non-function b (type int)

    fmt> !
    package main
    import "fmt"

    func b(a interface{})	{ fmt.Printf("You passed: %#v\n", a) }

    func noop(_ interface{}) {}

    func main() {
        a := 6;
        noop(a);
        b := 7;
        noop(b);
        fmt.Print();
    }

    fmt> -d
    fmt> !
    package main
    import "fmt"

    func noop(_ interface{}) {}

    func main() {
        a := 6;
        noop(a);
        b := 7;
        noop(b);
        fmt.Print();
    }

    fmt> func dump(a interface{}) { fmt.Printf("You passed: %#v\n", a); }
    fmt> dump("Phew, there we go.")
    You passed: "Phew, there we go."
    fmt> -d
    fmt> -c
    ! fmt> - fmt
    > + math
    ! math> println(math.Pi)
    +3.141593e+000
    ! math> + fmt
    ! math fmt> fmt.Println(math.Pi)
    3.1415927
    ! math fmt> 

TODO: Write automatic test with the above example session as input and expected output.
