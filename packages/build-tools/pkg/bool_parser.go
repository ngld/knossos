package pkg

import (
	"io"
	"regexp"
	"strings"

	"github.com/rotisserie/eris"
)

type BoolNode interface {
	Eval(map[string]bool) bool
}

type boolAnd struct {
	left  BoolNode
	right BoolNode
}

func (n boolAnd) Eval(vars map[string]bool) bool {
	return n.left.Eval(vars) && n.right.Eval(vars)
}

type boolOr struct {
	left  BoolNode
	right BoolNode
}

func (n boolOr) Eval(vars map[string]bool) bool {
	return n.left.Eval(vars) || n.right.Eval(vars)
}

type boolVar struct {
	name string
}

func (n boolVar) Eval(vars map[string]bool) bool {
	return vars[n.name]
}

type boolParen struct {
	node BoolNode
}

func (n boolParen) Eval(vars map[string]bool) bool {
	return n.node.Eval(vars)
}

var (
	letterSymRe = regexp.MustCompile("[a-zA-Z_]")
	numberRe    = regexp.MustCompile("[0-9]")
)

func ParseBoolExpr(input string) (BoolNode, error) {
	scanner := strings.NewReader(input)
	stack := make([]BoolNode, 0, 3)
	state := uint8(0)
	buffer := make([]rune, 0, 10)
	expect := ' '

	for {
		char, _, err := scanner.ReadRune()
		if err != nil {
			if eris.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		switch state {
		case 0:
			// looking for a var name
			if char == '(' {
				stack = append(stack, boolParen{node: nil})
				continue
			}

			// skip leading whitespace
			if (char == ' ' || char == '\t') && len(buffer) == 0 {
				continue
			}

			if char == ' ' || char == '\t' || char == '|' || char == '&' {
				// end of var name
				stack = append(stack, boolVar{name: string(buffer)})
				buffer = buffer[:0]

				state = 1
				expect = ' '
				if char == '|' || char == '&' {
					expect = char
				}

				continue
			}

			if !letterSymRe.MatchString(string(char)) {
				if len(buffer) == 0 || !numberRe.MatchString(string(char)) {
					return nil, eris.Errorf("expected a-z, A-Z or _ but found %c", char)
				}
			}

			buffer = append(buffer, char)
		case 1:
			// looking for an operator
			if expect == ' ' {
				if char == ' ' || char == '\t' {
					continue
				}

				if char == '&' || char == '|' {
					expect = char
					continue
				}

				return nil, eris.Errorf("expected operator (&& or ||) but found %c", char)
			}

			if char != expect {
				return nil, eris.Errorf("expected %c but found %c", expect, char)
			}

			top := len(stack) - 1
			switch char {
			case '&':
				stack[top] = boolAnd{left: stack[top]}
			case '|':
				stack[top] = boolOr{left: stack[top]}
			default:
				return nil, eris.Errorf("unreachable code, got operator %c", char)
			}

			state = 2
		case 2:
			if char == ' ' || char == '\t' {
				continue
			}

			if char == '(' {
				stack = append(stack, boolParen{node: nil})
				state = 0
				continue
			}

			err = scanner.UnreadRune()
			if err != nil {
				return nil, err
			}

			state = 3
		case 3:
			// looking for variable after operator
			top := len(stack) - 1

			if char == ' ' || char == '\t' || char == '|' || char == '&' || char == ')' {
				// end of var name
				varName := string(buffer)
				buffer = buffer[:0]

				switch node := stack[top].(type) {
				case boolAnd:
					node.right = boolVar{name: varName}
					stack[top] = node
				case boolOr:
					node.right = boolVar{name: varName}
					stack[top] = node
				default:
					return nil, eris.Errorf("unexpected stack top, expected boolAnd or boolOr but found %v", stack[top])
				}

				state = 1
				expect = ' '
				if char == '|' || char == '&' {
					expect = char
				}

				if char == ')' {
					preTop := len(stack) - 2
					_, ok := stack[preTop].(boolParen)
					if !ok {
						return nil, eris.Errorf("unexpected ), current node on stack is %v", stack[preTop])
					}

					// replace the paren node with the current top node
					if top > 1 {
						switch node := stack[top-2].(type) {
						case boolAnd:
							node.right = stack[top]
							stack[top-2] = node
							stack = stack[:preTop]
						case boolOr:
							node.right = stack[top]
							stack[top-2] = node
							stack = stack[:preTop]
						default:
							stack[preTop] = stack[top]
							stack = stack[:top]
						}
					} else {
						stack[preTop] = stack[top]
						stack = stack[:top]
					}
				}

				continue
			}

			if !letterSymRe.MatchString(string(char)) {
				if len(buffer) == 0 || !numberRe.MatchString(string(char)) {
					return nil, eris.Errorf("expected a-z, A-Z or _ but found %c", char)
				}
			}

			buffer = append(buffer, char)
		}
	}

	if len(stack) > 1 {
		return nil, eris.Errorf("more than one node left on stack: %v", stack)
	}

	if len(buffer) > 0 {
		switch node := stack[0].(type) {
		case boolAnd:
			node.right = boolVar{name: string(buffer)}
			stack[0] = node
		case boolOr:
			node.right = boolVar{name: string(buffer)}
			stack[0] = node
		default:
			return nil, eris.Errorf("found var string after node %v", stack[0])
		}
	}

	if len(stack) == 0 {
		return nil, eris.New("no expression found in input")
	}

	return stack[0], nil
}
