// Copyright (c) 2016, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package sh

import (
	"reflect"
	"strings"
	"testing"
)

func lit(s string) Lit { return Lit{Value: s} }
func lits(strs ...string) []Node {
	l := make([]Node, len(strs))
	for i, s := range strs {
		l[i] = lit(s)
	}
	return l
}

func word(ns ...Node) Word  { return Word{Parts: ns} }
func litWord(s string) Word { return word(lits(s)...) }
func litWords(strs ...string) []Word {
	l := make([]Word, 0, len(strs))
	for _, s := range strs {
		l = append(l, litWord(s))
	}
	return l
}

func litCmd(strs ...string) Command {
	return Command{Args: litWords(strs...)}
}

func stmt(n Node) Stmt { return Stmt{Node: n} }
func stmts(ns ...Node) []Stmt {
	l := make([]Stmt, len(ns))
	for i, n := range ns {
		l[i] = stmt(n)
	}
	return l
}

func litStmt(strs ...string) Stmt { return stmt(litCmd(strs...)) }
func litStmts(strs ...string) []Stmt {
	l := make([]Stmt, len(strs))
	for i, s := range strs {
		l[i] = litStmt(s)
	}
	return l
}

func dblQuoted(ns ...Node) DblQuoted  { return DblQuoted{Parts: ns} }
func block(stmts ...Stmt) Block       { return Block{Stmts: stmts} }
func cmdSubst(stmts ...Stmt) CmdSubst { return CmdSubst{Stmts: stmts} }

var tests = []struct {
	ins  []string
	want interface{}
}{
	{
		[]string{"", " ", "\n", "# foo"},
		nil,
	},
	{
		[]string{"foo", "foo ", " foo", "foo # bar"},
		litCmd("foo"),
	},
	{
		[]string{"foo; bar", "foo; bar;", "foo;bar;", "\nfoo\nbar\n"},
		[]Node{
			litCmd("foo"),
			litCmd("bar"),
		},
	},
	{
		[]string{"foo a b", " foo  a  b ", "foo \\\n a b"},
		litCmd("foo", "a", "b"),
	},
	{
		[]string{"foobar", "foo\\\nbar"},
		litCmd("foobar"),
	},
	{
		[]string{"foo'bar'"},
		litCmd("foo'bar'"),
	},
	{
		[]string{"(foo)", "(foo;)", "(\nfoo\n)"},
		Subshell{Stmts: litStmts("foo")},
	},
	{
		[]string{"{ foo; }", "{foo;}", "{\nfoo\n}"},
		block(litStmt("foo")),
	},
	{
		[]string{
			"if a; then b; fi",
			"if a\nthen\nb\nfi",
		},
		IfStmt{
			Cond:      litStmt("a"),
			ThenStmts: litStmts("b"),
		},
	},
	{
		[]string{
			"if a; then b; else c; fi",
			"if a\nthen b\nelse\nc\nfi",
		},
		IfStmt{
			Cond:      litStmt("a"),
			ThenStmts: litStmts("b"),
			ElseStmts: litStmts("c"),
		},
	},
	{
		[]string{
			"if a; then a; elif b; then b; elif c; then c; else d; fi",
			"if a\nthen a\nelif b\nthen b\nelif c\nthen c\nelse\nd\nfi",
		},
		IfStmt{
			Cond:      litStmt("a"),
			ThenStmts: litStmts("a"),
			Elifs: []Elif{
				{
					Cond:      litStmt("b"),
					ThenStmts: litStmts("b"),
				},
				{
					Cond:      litStmt("c"),
					ThenStmts: litStmts("c"),
				},
			},
			ElseStmts: litStmts("d"),
		},
	},
	{
		[]string{"while a; do b; done", "while a\ndo\nb\ndone"},
		WhileStmt{
			Cond:    litStmt("a"),
			DoStmts: litStmts("b"),
		},
	},
	{
		[]string{
			"for i in 1 2 3; do echo $i; done",
			"for i in 1 2 3\ndo echo $i\ndone",
		},
		ForStmt{
			Name:     lit("i"),
			WordList: litWords("1", "2", "3"),
			DoStmts: stmts(Command{Args: []Word{
				litWord("echo"),
				word(ParamExp{Short: true, Text: "i"}),
			}}),
		},
	},
	{
		[]string{`echo ' ' "foo bar"`},
		Command{Args: []Word{
			litWord("echo"),
			litWord("' '"),
			word(dblQuoted(lits("foo bar")...)),
		}},
	},
	{
		[]string{`"foo \" bar"`},
		Command{Args: []Word{
			word(dblQuoted(lits(`foo \" bar`)...)),
		}},
	},
	{
		[]string{"\">foo\" \"\nbar\""},
		Command{Args: []Word{
			word(dblQuoted(lits(">foo")...)),
			word(dblQuoted(lits("\nbar")...)),
		}},
	},
	{
		[]string{`foo \" bar`},
		litCmd(`foo`, `\"`, `bar`),
	},
	{
		[]string{"s{s s=s"},
		litCmd("s{s", "s=s"),
	},
	{
		[]string{"foo && bar", "foo&&bar", "foo &&\nbar"},
		BinaryExpr{
			Op: LAND,
			X:  litStmt("foo"),
			Y:  litStmt("bar"),
		},
	},
	{
		[]string{"foo || bar", "foo||bar", "foo ||\nbar"},
		BinaryExpr{
			Op: LOR,
			X:  litStmt("foo"),
			Y:  litStmt("bar"),
		},
	},
	{
		[]string{"if a; then b; fi || while a; do b; done"},
		BinaryExpr{
			Op: LOR,
			X: stmt(IfStmt{
				Cond:      litStmt("a"),
				ThenStmts: litStmts("b"),
			}),
			Y: stmt(WhileStmt{
				Cond:    litStmt("a"),
				DoStmts: litStmts("b"),
			}),
		},
	},
	{
		[]string{"foo && bar1 || bar2"},
		BinaryExpr{
			Op: LAND,
			X:  litStmt("foo"),
			Y: stmt(BinaryExpr{
				Op: LOR,
				X:  litStmt("bar1"),
				Y:  litStmt("bar2"),
			}),
		},
	},
	{
		[]string{"foo | bar", "foo|bar"},
		BinaryExpr{
			Op: OR,
			X:  litStmt("foo"),
			Y:  litStmt("bar"),
		},
	},
	{
		[]string{"foo | bar | extra"},
		BinaryExpr{
			Op: OR,
			X:  litStmt("foo"),
			Y: stmt(BinaryExpr{
				Op: OR,
				X:  litStmt("bar"),
				Y:  litStmt("extra"),
			}),
		},
	},
	{
		[]string{
			"foo() { a; b; }",
			"foo() {\na\nb\n}",
			"foo ( ) {\na\nb\n}",
		},
		FuncDecl{
			Name: lit("foo"),
			Body: stmt(block(litStmts("a", "b")...)),
		},
	},
	{
		[]string{
			"foo >a >>b <c",
			"foo > a >> b < c",
			">a >>b foo <c",
		},
		Stmt{
			Node: litCmd("foo"),
			Redirs: []Redirect{
				{Op: RDROUT, Word: litWord("a")},
				{Op: APPEND, Word: litWord("b")},
				{Op: RDRIN, Word: litWord("c")},
			},
		},
	},
	{
		[]string{
			"foo bar >a",
			"foo >a bar",
		},
		Stmt{
			Node: litCmd("foo", "bar"),
			Redirs: []Redirect{
				{Op: RDROUT, Word: litWord("a")},
			},
		},
	},
	{
		[]string{"foo <<EOF\nbar\nEOF"},
		Stmt{
			Node: litCmd("foo"),
			Redirs: []Redirect{
				{Op: HEREDOC, Word: litWord("EOF\nbar\nEOF")},
			},
		},
	},
	{
		[]string{"foo <<FOOBAR\nbar\nFOOBAR"},
		Stmt{
			Node: litCmd("foo"),
			Redirs: []Redirect{
				{Op: HEREDOC, Word: litWord("FOOBAR\nbar\nFOOBAR")},
			},
		},
	},
	{
		[]string{"foo >&2 <&0 2>file"},
		Stmt{
			Node: litCmd("foo"),
			Redirs: []Redirect{
				{Op: DPLOUT, Word: litWord("2")},
				{Op: DPLIN, Word: litWord("0")},
				{Op: RDROUT, N: lit("2"), Word: litWord("file")},
			},
		},
	},
	{
		[]string{"a >f1; b >f2"},
		[]Stmt{
			{
				Node:   litCmd("a"),
				Redirs: []Redirect{{Op: RDROUT, Word: litWord("f1")}},
			},
			{
				Node:   litCmd("b"),
				Redirs: []Redirect{{Op: RDROUT, Word: litWord("f2")}},
			},
		},
	},
	{
		[]string{"foo &", "foo&"},
		Stmt{
			Node:       litCmd("foo"),
			Background: true,
		},
	},
	{
		[]string{"if foo; then bar; fi >/dev/null &"},
		Stmt{
			Node: IfStmt{
				Cond:      litStmt("foo"),
				ThenStmts: litStmts("bar"),
			},
			Redirs: []Redirect{
				{Op: RDROUT, Word: litWord("/dev/null")},
			},
			Background: true,
		},
	},
	{
		[]string{"echo foo#bar"},
		litCmd("echo", "foo#bar"),
	},
	{
		[]string{"echo $(foo bar)"},
		Command{Args: []Word{
			litWord("echo"),
			word(cmdSubst(litStmt("foo", "bar"))),
		}},
	},
	{
		[]string{"echo $(foo | bar)"},
		Command{Args: []Word{
			litWord("echo"),
			word(cmdSubst(stmt(BinaryExpr{
				Op: OR,
				X:  litStmt("foo"),
				Y:  litStmt("bar"),
			}))),
		}},
	},
	{
		[]string{`echo "$foo"`},
		Command{Args: []Word{
			litWord("echo"),
			word(dblQuoted(ParamExp{Short: true, Text: "foo"})),
		}},
	},
	{
		[]string{`$@ $# $$`},
		Command{Args: []Word{
			word(ParamExp{Short: true, Text: "@"}),
			word(ParamExp{Short: true, Text: "#"}),
			word(ParamExp{Short: true, Text: "$"}),
		}},
	},
	{
		[]string{`echo "${foo}"`},
		Command{Args: []Word{
			litWord("echo"),
			word(dblQuoted(ParamExp{Text: "foo"})),
		}},
	},
	{
		[]string{`echo "$(foo)"`},
		Command{Args: []Word{
			litWord("echo"),
			word(dblQuoted(cmdSubst(litStmt("foo")))),
		}},
	},
	{
		[]string{`echo '${foo}'`},
		litCmd("echo", "'${foo}'"),
	},
	{
		[]string{"echo ${foo bar}"},
		Command{Args: []Word{
			litWord("echo"),
			word(ParamExp{Text: "foo bar"}),
		}},
	},
	{
		[]string{"echo $(($x-1))"},
		Command{Args: []Word{
			litWord("echo"),
			word(ArithmExp{Text: "$x-1"}),
		}},
	},
	{
		[]string{"echo foo$bar"},
		Command{Args: []Word{
			litWord("echo"),
			word(lit("foo"), ParamExp{Short: true, Text: "bar"}),
		}},
	},
	{
		[]string{"echo foo$(bar)"},
		Command{Args: []Word{
			litWord("echo"),
			word(lit("foo"), cmdSubst(litStmt("bar"))),
		}},
	},
	{
		[]string{"echo foo${bar bar}"},
		Command{Args: []Word{
			litWord("echo"),
			word(lit("foo"), ParamExp{Text: "bar bar"}),
		}},
	},
	{
		[]string{"echo 'foo${bar'"},
		litCmd("echo", "'foo${bar'"),
	},
	{
		[]string{"(foo); bar"},
		[]Node{
			Subshell{Stmts: litStmts("foo")},
			litCmd("bar"),
		},
	},
	{
		[]string{"a=\"\nbar\""},
		Command{Args: []Word{
			word(lit("a="), dblQuoted(lits("\nbar")...)),
		}},
	},
	{
		[]string{
			"case $i in 1) foo;; 2 | 3*) bar; esac",
			"case $i in 1) foo;; 2 | 3*) bar;; esac",
			"case $i in\n1)\nfoo\n;;\n2 | 3*)\nbar\n;;\nesac",
		},
		CaseStmt{
			Word: word(ParamExp{Short: true, Text: "i"}),
			List: []PatternList{
				{
					Patterns: litWords("1"),
					Stmts:    litStmts("foo"),
				},
				{
					Patterns: litWords("2", "3*"),
					Stmts:    litStmts("bar"),
				},
			},
		},
	},
	{
		[]string{"foo | while read a; do b; done"},
		BinaryExpr{
			Op: OR,
			X:  litStmt("foo"),
			Y: stmt(WhileStmt{
				Cond:    litStmt("read", "a"),
				DoStmts: litStmts("b"),
			}),
		},
	},
	{
		[]string{"while read l; do foo || bar; done"},
		WhileStmt{
			Cond: litStmt("read", "l"),
			DoStmts: stmts(BinaryExpr{
				Op: LOR,
				X:  litStmt("foo"),
				Y:  litStmt("bar"),
			}),
		},
	},
	{
		[]string{"echo if while"},
		litCmd("echo", "if", "while"),
	},
	{
		[]string{"echo ${foo}if"},
		Command{Args: []Word{
			litWord("echo"),
			word(ParamExp{Text: "foo"}, lit("if")),
		}},
	},
	{
		[]string{"echo $if"},
		Command{Args: []Word{
			litWord("echo"),
			word(ParamExp{Short: true, Text: "if"}),
		}},
	},
}

func wantedProg(v interface{}) (p Prog) {
	switch x := v.(type) {
	case []Stmt:
		p.Stmts = x
	case Stmt:
		p.Stmts = append(p.Stmts, x)
	case []Node:
		for _, n := range x {
			p.Stmts = append(p.Stmts, stmt(n))
		}
	case Node:
		p.Stmts = append(p.Stmts, stmt(x))
	}
	return
}

func setPos(v interface{}, p Position) Node {
	switch x := v.(type) {
	case []Stmt:
		for i := range x {
			setPos(&x[i], p)
		}
	case *Stmt:
		x.Position = p
		x.Node = setPos(x.Node, p)
		for i := range x.Redirs {
			x.Redirs[i].OpPos = p
			setPos(&x.Redirs[i].N, p)
			setPos(&x.Redirs[i].Word, p)
		}
	case Command:
		setPos(x.Args, p)
		return x
	case []Word:
		for i := range x {
			setPos(&x[i], p)
		}
	case *Word:
		setPos(x.Parts, p)
	case []Node:
		for i := range x {
			x[i] = setPos(x[i], p)
		}
	case *Lit:
		x.ValuePos = p
	case Lit:
		x.ValuePos = p
		return x
	case Subshell:
		x.Lparen = p
		x.Rparen = p
		setPos(x.Stmts, p)
		return x
	case Block:
		x.Lbrace = p
		x.Rbrace = p
		setPos(x.Stmts, p)
		return x
	case IfStmt:
		x.If = p
		x.Fi = p
		setPos(&x.Cond, p)
		setPos(x.ThenStmts, p)
		for i := range x.Elifs {
			x.Elifs[i].Elif = p
			setPos(&x.Elifs[i].Cond, p)
			setPos(x.Elifs[i].ThenStmts, p)
		}
		setPos(x.ElseStmts, p)
		return x
	case WhileStmt:
		x.While = p
		x.Done = p
		setPos(&x.Cond, p)
		setPos(x.DoStmts, p)
		return x
	case ForStmt:
		x.For = p
		x.Done = p
		setPos(&x.Name, p)
		setPos(x.WordList, p)
		setPos(x.DoStmts, p)
		return x
	case DblQuoted:
		x.Quote = p
		setPos(x.Parts, p)
		return x
	case BinaryExpr:
		x.OpPos = p
		setPos(&x.X, p)
		setPos(&x.Y, p)
		return x
	case FuncDecl:
		setPos(&x.Name, p)
		setPos(&x.Body, p)
		return x
	case ParamExp:
		x.Exp = p
		return x
	case ArithmExp:
		x.Exp = p
		return x
	case CmdSubst:
		x.Exp = p
		setPos(x.Stmts, p)
		return x
	case CaseStmt:
		x.Case = p
		x.Esac = p
		setPos(&x.Word, p)
		for _, pl := range x.List {
			setPos(pl.Patterns, p)
			setPos(pl.Stmts, p)
		}
		return x
	default:
		panic(v)
	}
	return nil
}

func TestNodePos(t *testing.T) {
	p := Position{
		Line: 12,
		Col:  34,
	}
	for _, c := range tests {
		want := wantedProg(c.want)
		setPos(want.Stmts, p)
		for _, s := range want.Stmts {
			if s.Pos() != p {
				t.Fatalf("Found unexpected position in %v", s)
			}
			n := s.Node
			if n.Pos() != p {
				t.Fatalf("Found unexpected position in %v", n)
			}
		}
	}
}

func TestParseAST(t *testing.T) {
	for _, c := range tests {
		want := wantedProg(c.want)
		setPos(want.Stmts, Position{})
		for _, in := range c.ins {
			r := strings.NewReader(in)
			got, err := Parse(r, "")
			if err != nil {
				t.Fatalf("Unexpected error in %q: %v", in, err)
			}
			setPos(got.Stmts, Position{})
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("AST mismatch in %q\nwant: %s\ngot:  %s\ndumps:\n%#v\n%#v",
					in, want.String(), got.String(), want, got)
			}
		}
	}
}

func TestPrintAST(t *testing.T) {
	for _, c := range tests {
		in := wantedProg(c.want)
		want := c.ins[0]
		got := in.String()
		if got != want {
			t.Fatalf("AST print mismatch\nwant: %s\ngot:  %s",
				want, got)
		}
	}
}
