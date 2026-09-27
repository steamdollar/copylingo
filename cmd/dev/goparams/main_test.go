package main

import "testing"

func TestFormatParameters(t *testing.T) {
	tests := []struct {
		name, source, want string
		wantErr            bool
	}{
		{
			name:   "function parameters and call arguments",
			source: "package p\nfunc f(a int,b string){ g(a,b) }\n",
			want:   "package p\n\nfunc f(\n\ta int,\n\tb string,\n) {\n\tg(\n\t\ta,\n\t\tb,\n\t)\n}\n",
		},
		{
			name:   "nested variadic and generic function",
			source: "package p\nfunc f[T any](first T, rest ...any){ consume(makePair(first, 1), rest...) }\n",
			want:   "package p\n\nfunc f[T any](\n\tfirst T,\n\trest ...any,\n) {\n\tconsume(\n\t\tmakePair(\n\t\t\tfirst,\n\t\t\t1,\n\t\t),\n\t\trest...,\n\t)\n}\n",
		},
		{
			name:   "three nested calls and adjacent signatures",
			source: "package p\nfunc first(a,b int) {}\nfunc second(){ outer(inner(more(1,2), leaf(3,4)), sibling(5,6)) }\nfunc third(c,d string) {}\n",
			want:   "package p\n\nfunc first(\n\ta,\n\tb int,\n) {\n}\nfunc second() {\n\touter(\n\t\tinner(\n\t\t\tmore(\n\t\t\t\t1,\n\t\t\t\t2,\n\t\t\t),\n\t\t\tleaf(\n\t\t\t\t3,\n\t\t\t\t4,\n\t\t\t),\n\t\t),\n\t\tsibling(\n\t\t\t5,\n\t\t\t6,\n\t\t),\n\t)\n}\nfunc third(\n\tc,\n\td string,\n) {\n}\n",
		},
		{
			name:   "grouped parameter names",
			source: "package p\nfunc f(a,b int) {}\n",
			want:   "package p\n\nfunc f(\n\ta,\n\tb int,\n) {\n}\n",
		},
		{
			name:   "comments strings and nested composite commas",
			source: "package p\nfunc f() { call(\"a,b\", /* comma, */ []int{1,2}) }\n",
			want:   "package p\n\nfunc f() {\n\tcall(\n\t\t\"a,b\",\n\t\t/* comma, */\n\t\t[]int{1, 2},\n\t)\n}\n",
		},
		{
			name:   "zero and one item stay on one line",
			source: "package p\nfunc zero() {}\nfunc one(x int) { call(x) }\n",
			want:   "package p\n\nfunc zero()     {}\nfunc one(x int) { call(x) }\n",
		},
		{
			name:    "invalid source returns no output",
			source:  "package p\nfunc broken( {\n",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				got, err := formatParameters([]byte(test.source))
				if test.wantErr {
					if err == nil || got != nil {
						t.Fatalf(
							"formatParameters() = %q, %v; want nil output and an error",
							got,
							err,
						)
					}
					return
				}
				if err != nil {
					t.Fatalf(
						"formatParameters() error = %v",
						err,
					)
				}
				if string(got) != test.want {
					t.Fatalf(
						"formatParameters() =\n%s\nwant:\n%s",
						got,
						test.want,
					)
				}
				again, err := formatParameters(got)
				if err != nil || string(again) != string(got) {
					t.Fatalf(
						"second format =\n%s\nerror = %v",
						again,
						err,
					)
				}
			},
		)
	}
}
