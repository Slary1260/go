// UNREVIEWED
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements printing of expressions.

package types2

import (
	"bytes"
	"cmd/compile/internal/syntax"
)

// ExprString returns the (possibly shortened) string representation for x.
// Shortened representations are suitable for user interfaces but may not
// necessarily follow Go syntax.
func ExprString(x syntax.Expr) string {
	var buf bytes.Buffer
	WriteExpr(&buf, x)
	return buf.String()
}

// WriteExpr writes the (possibly shortened) string representation for x to buf.
// Shortened representations are suitable for user interfaces but may not
// necessarily follow Go syntax.
func WriteExpr(buf *bytes.Buffer, x syntax.Expr) {
	// The AST preserves source-level parentheses so there is
	// no need to introduce them here to correct for different
	// operator precedences. (This assumes that the AST was
	// generated by a Go parser.)

	// TODO(gri): This assumption is not correct - we need to recreate
	//            parentheses in expressions.

	switch x := x.(type) {
	default:
		buf.WriteString("(ast: bad expr)") // nil, syntax.BadExpr, syntax.KeyValueExpr

	case *syntax.Name:
		buf.WriteString(x.Value)

	case *syntax.DotsType:
		buf.WriteString("...")
		if x.Elem != nil {
			WriteExpr(buf, x.Elem)
		}

	case *syntax.BasicLit:
		buf.WriteString(x.Value)

	case *syntax.FuncLit:
		buf.WriteByte('(')
		WriteExpr(buf, x.Type)
		buf.WriteString(" literal)") // shortened

	case *syntax.CompositeLit:
		buf.WriteByte('(')
		WriteExpr(buf, x.Type)
		buf.WriteString(" literal)") // shortened

	case *syntax.ParenExpr:
		buf.WriteByte('(')
		WriteExpr(buf, x.X)
		buf.WriteByte(')')

	case *syntax.SelectorExpr:
		WriteExpr(buf, x.X)
		buf.WriteByte('.')
		buf.WriteString(x.Sel.Value)

	case *syntax.IndexExpr:
		WriteExpr(buf, x.X)
		buf.WriteByte('[')
		WriteExpr(buf, x.Index) // x.Index may be a *ListExpr
		buf.WriteByte(']')

	case *syntax.SliceExpr:
		WriteExpr(buf, x.X)
		buf.WriteByte('[')
		if x.Index[0] != nil {
			WriteExpr(buf, x.Index[0])
		}
		buf.WriteByte(':')
		if x.Index[1] != nil {
			WriteExpr(buf, x.Index[1])
		}
		if x.Full {
			buf.WriteByte(':')
			if x.Index[2] != nil {
				WriteExpr(buf, x.Index[2])
			}
		}
		buf.WriteByte(']')

	case *syntax.AssertExpr:
		WriteExpr(buf, x.X)
		buf.WriteString(".(")
		WriteExpr(buf, x.Type)
		buf.WriteByte(')')

	case *syntax.CallExpr:
		WriteExpr(buf, x.Fun)
		buf.WriteByte('(')
		writeExprList(buf, x.ArgList)
		if x.HasDots {
			buf.WriteString("...")
		}
		buf.WriteByte(')')

	case *syntax.ListExpr:
		writeExprList(buf, x.ElemList)

	case *syntax.Operation:
		// TODO(gri) This would be simpler if x.X == nil meant unary expression.
		if x.Y == nil {
			// unary expression
			buf.WriteString(x.Op.String())
			WriteExpr(buf, x.X)
		} else {
			// binary expression
			WriteExpr(buf, x.X)
			buf.WriteByte(' ')
			buf.WriteString(x.Op.String())
			buf.WriteByte(' ')
			WriteExpr(buf, x.Y)
		}

		// case *ast.StarExpr:
		// 	buf.WriteByte('*')
		// 	WriteExpr(buf, x.X)

		// case *ast.UnaryExpr:
		// 	buf.WriteString(x.Op.String())
		// 	WriteExpr(buf, x.X)

		// case *ast.BinaryExpr:
		// 	WriteExpr(buf, x.X)
		// 	buf.WriteByte(' ')
		// 	buf.WriteString(x.Op.String())
		// 	buf.WriteByte(' ')
		// 	WriteExpr(buf, x.Y)

	case *syntax.ArrayType:
		if x.Len == nil {
			buf.WriteString("[...]")
		} else {
			buf.WriteByte('[')
			WriteExpr(buf, x.Len)
			buf.WriteByte(']')
		}
		WriteExpr(buf, x.Elem)

	case *syntax.SliceType:
		buf.WriteString("[]")
		WriteExpr(buf, x.Elem)

	case *syntax.StructType:
		buf.WriteString("struct{")
		writeFieldList(buf, x.FieldList, "; ", false)
		buf.WriteByte('}')

	case *syntax.FuncType:
		buf.WriteString("func")
		writeSigExpr(buf, x)

	case *syntax.InterfaceType:
		// separate type list types from method list
		// TODO(gri) we can get rid of this extra code if writeExprList does the separation
		var types []syntax.Expr
		var methods []*syntax.Field
		for _, f := range x.MethodList {
			if f.Name != nil && f.Name.Value == "type" {
				// type list type
				types = append(types, f.Type)
			} else {
				// method or embedded interface
				methods = append(methods, f)
			}
		}

		buf.WriteString("interface{")
		writeFieldList(buf, methods, "; ", true)
		if len(types) > 0 {
			if len(methods) > 0 {
				buf.WriteString("; ")
			}
			buf.WriteString("type ")
			writeExprList(buf, types)
		}
		buf.WriteByte('}')

	case *syntax.MapType:
		buf.WriteString("map[")
		WriteExpr(buf, x.Key)
		buf.WriteByte(']')
		WriteExpr(buf, x.Value)

	case *syntax.ChanType:
		var s string
		switch x.Dir {
		case syntax.SendOnly:
			s = "chan<- "
		case syntax.RecvOnly:
			s = "<-chan "
		default:
			s = "chan "
		}
		buf.WriteString(s)
		if e, _ := x.Elem.(*syntax.ChanType); x.Dir != syntax.SendOnly && e != nil && e.Dir == syntax.RecvOnly {
			// don't print chan (<-chan T) as chan <-chan T (but chan<- <-chan T is ok)
			buf.WriteByte('(')
			WriteExpr(buf, x.Elem)
			buf.WriteByte(')')
		} else {
			WriteExpr(buf, x.Elem)
		}
	}
}

func writeSigExpr(buf *bytes.Buffer, sig *syntax.FuncType) {
	buf.WriteByte('(')
	writeFieldList(buf, sig.ParamList, ", ", false)
	buf.WriteByte(')')

	res := sig.ResultList
	n := len(res)
	if n == 0 {
		// no result
		return
	}

	buf.WriteByte(' ')
	if n == 1 && res[0].Name == nil {
		// single unnamed result
		WriteExpr(buf, res[0].Type)
		return
	}

	// multiple or named result(s)
	buf.WriteByte('(')
	writeFieldList(buf, res, ", ", false)
	buf.WriteByte(')')
}

func writeFieldList(buf *bytes.Buffer, list []*syntax.Field, sep string, iface bool) {
	for i := 0; i < len(list); {
		f := list[i]
		if i > 0 {
			buf.WriteString(sep)
		}

		// if we don't have a name, we have an embedded type
		if f.Name == nil {
			WriteExpr(buf, f.Type)
			i++
			continue
		}

		// types of interface methods consist of signatures only
		if sig, _ := f.Type.(*syntax.FuncType); sig != nil && iface {
			buf.WriteString(f.Name.Value)
			writeSigExpr(buf, sig)
			i++
			continue
		}

		// write the type only once for a sequence of fields with the same type
		t := f.Type
		buf.WriteString(f.Name.Value)
		for i++; i < len(list) && list[i].Type == t; i++ {
			buf.WriteString(", ")
			buf.WriteString(list[i].Name.Value)
		}
		buf.WriteByte(' ')
		WriteExpr(buf, t)
	}
}

func writeExprList(buf *bytes.Buffer, list []syntax.Expr) {
	for i, x := range list {
		if i > 0 {
			buf.WriteString(", ")
		}
		WriteExpr(buf, x)
	}
}