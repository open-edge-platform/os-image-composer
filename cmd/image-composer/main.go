package main

import (
    "fmt"
    "io/ioutil"
    "os"

    "github.com/intel-innersource/os.linux.tiberos.os-curation-tool/pkg/validate"
)

func main() {
    if len(os.Args) != 2 {
        fmt.Fprintf(os.Stderr, "Usage: %s <input.json>\n", os.Args[0])
        os.Exit(1)
    }
    path := os.Args[1]
    data, err := ioutil.ReadFile(path)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
        os.Exit(1)
    }
    if err := validate.ValidateJSON(data); err != nil {
        fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
        os.Exit(1)
    }
    fmt.Println("valid input")
}