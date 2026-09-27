package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"io"
	"os"
	"sort"
	"strings"
)

type parameterList struct {
	open, close int
	count       int
	commas      []int
}

type appliedEdit struct {
	end, delta int
}

func main() {
	source, err := io.ReadAll(os.Stdin)
	if err == nil {
		source, err = formatParameters(source)
	}
	if err != nil {
		fmt.Fprintln(
			os.Stderr,
			err,
		)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(source); err != nil {
		fmt.Fprintln(
			os.Stderr,
			err,
		)
		os.Exit(1)
	}
}

func formatParameters(source []byte) ([]byte, error) {
	// gofmt can reposition comments between arguments; return a stable layout on the first save.
	seen := map[string]bool{}
	for {
		if seen[string(source)] {
			return nil, fmt.Errorf("formatter did not converge")
		}
		seen[string(source)] = true
		formatted, err := formatPass(source)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(
			formatted,
			source,
		) {
			return formatted, nil
		}
		source = formatted
	}
}

func formatPass(source []byte) ([]byte, error) {
	formatted, err := format.Source(source)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(
		fset,
		"stdin.go",
		formatted,
		parser.ParseComments,
	)
	if err != nil {
		return nil, err
	}
	var lists []parameterList
	addList := func(
		open,
		close token.Pos,
		count int,
	) {
		if count < 2 {
			return
		}
		lists = append(
			lists,
			parameterList{
				open:  fset.Position(open).Offset,
				close: fset.Position(close).Offset,
				count: count,
			},
		)
	}
	ast.Inspect(
		file,
		func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.FuncType:
				count := 0
				for _, field := range node.Params.List {
					if len(field.Names) == 0 {
						count++
					} else {
						count += len(field.Names)
					}
				}
				addList(
					node.Params.Opening,
					node.Params.Closing,
					count,
				)
			case *ast.CallExpr:
				addList(
					node.Lparen,
					node.Rparen,
					len(node.Args),
				)
			}
			return true
		},
	)
	for index := range lists {
		lists[index].commas = topLevelCommas(
			formatted,
			lists[index].open+1,
			lists[index].close,
		)
	}
	// Inner spans are rewritten first, so parent slices already include nested formatting.
	// ponytail: quadratic edits per file; use a single edit pass if save latency becomes noticeable.
	sort.Slice(
		lists,
		func(
			i,
			j int,
		) bool {
			return lists[i].open > lists[j].open
		},
	)
	var applied []appliedEdit
	for _, list := range lists {
		open := adjustedOffset(
			list.open,
			applied,
		)
		close := adjustedOffset(
			list.close,
			applied,
		)
		commas := make(
			[]int,
			len(list.commas),
		)
		for i, comma := range list.commas {
			commas[i] = adjustedOffset(
				comma,
				applied,
			)
		}
		replacement, err := multilineList(
			formatted,
			open,
			close,
			list.count,
			commas,
		)
		if err != nil {
			return nil, err
		}
		formatted = append(
			append(
				append(
					[]byte(nil),
					formatted[:open]...,
				),
				replacement...,
			),
			formatted[close+1:]...,
		)
		applied = append(
			applied,
			appliedEdit{
				end:   list.close + 1,
				delta: len(replacement) - (close + 1 - open),
			},
		)
	}
	return format.Source(formatted)
}

func topLevelCommas(
	source []byte,
	start,
	end int,
) []int {
	// Go tokens keep commas inside strings, comments and nested expressions out of this list.
	inside := source[start:end]
	file := token.NewFileSet().AddFile(
		"",
		-1,
		len(inside),
	)
	var lexer scanner.Scanner
	lexer.Init(
		file,
		inside,
		nil,
		scanner.ScanComments,
	)
	var parens, brackets, braces int
	var commas []int
	for {
		position, tokenType, _ := lexer.Scan()
		if tokenType == token.EOF {
			return commas
		}
		if parens == 0 && brackets == 0 && braces == 0 && tokenType == token.COMMA {
			commas = append(
				commas,
				start+file.Offset(position),
			)
			continue
		}
		switch tokenType {
		case token.LPAREN:
			parens++
		case token.RPAREN:
			parens--
		case token.LBRACK:
			brackets++
		case token.RBRACK:
			brackets--
		case token.LBRACE:
			braces++
		case token.RBRACE:
			braces--
		}
	}
}

func adjustedOffset(
	offset int,
	edits []appliedEdit,
) int {
	// Compare original coordinates and add only each edit's incremental size change.
	originalOffset := offset
	delta := 0
	for _, edit := range edits {
		if edit.end <= originalOffset {
			delta += edit.delta
		}
	}
	return originalOffset + delta
}

func multilineList(
	source []byte,
	open,
	close,
	count int,
	commas []int,
) ([]byte, error) {
	if len(commas) < count-1 {
		return nil, fmt.Errorf("could not locate separators in parameter list")
	}
	var output bytes.Buffer
	output.WriteByte('(')
	output.WriteByte('\n')
	start := open + 1
	for index := 0; index < count; index++ {
		end := close
		if index < count-1 {
			end = commas[index]
		} else if len(commas) >= count {
			end = commas[count-1]
		}
		item := strings.TrimSpace(string(source[start:end]))
		if item == "" {
			return nil, fmt.Errorf("empty item in parameter list")
		}
		output.WriteString(item)
		output.WriteString(",\n")
		if index < count-1 {
			start = commas[index] + 1
		}
	}
	if len(commas) >= count {
		trailing := strings.TrimSpace(string(source[commas[count-1]+1 : close]))
		if trailing != "" {
			output.WriteString(trailing)
			output.WriteByte('\n')
		}
	}
	output.WriteByte(')')
	return output.Bytes(), nil
}
