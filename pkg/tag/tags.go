package tag

import (
	"log/slog"
	"slices"
	"strings"

	"github.com/larynjahor/spd/container"
)

func New() *Evaler {
	return &Evaler{}
}

type Evaler struct{}

func (p *Evaler) Eval(s string, tags []string) bool {
	logger := slog.With(slog.String("build directive", s))

	out := container.NewStack[string]()
	ops := container.NewStack[string]()

	for _, token := range strings.Fields(s) {
		temp := []string{}

		tempString := ""

		for i, ch := range token {
			switch ch {
			case '!', '(', ')':
				if tempString != "" {
					temp = append(temp, tempString)
					tempString = ""
				}

				temp = append(temp, string(ch))
			default:
				tempString += string(ch)
				if i == len(token)-1 {
					temp = append(temp, tempString)
				}
			}
		}

		for _, t := range temp {
			switch t {
			case "!":
				ops.Push(t)
			case "&&":
				for !ops.Empty() && !(ops.Top() == "&&" || ops.Top() == "||" || ops.Top() == "(") {
					out.Push(ops.Pop())
				}

				ops.Push(t)
			case "||":
				for !ops.Empty() && !(ops.Top() == "||" || ops.Top() == "(") {
					out.Push(ops.Pop())
				}

				ops.Push(t)
			case "(":
				ops.Push("(")
			case ")":
				for !ops.Empty() {
					cur := ops.Pop()
					if cur == "(" {
						break
					}

					out.Push(cur)
				}
			default:
				out.Push(t)
			}
		}

	}

	for !ops.Empty() {
		out.Push(ops.Pop())
	}

	eval := container.NewStack[bool]()

	for _, t := range out.Values() {
		switch t {
		case "!":
			if eval.Empty() {
				logger.Error("no operand for !")
				return false
			}

			eval.Push(!eval.Pop())
		case "||", "&&":
			if eval.Empty() {
				logger.Error("no operand for || or &&")
				return false
			}

			first := eval.Pop()

			if eval.Empty() {
				logger.Error("no operand for || or &&")
				return false
			}

			second := eval.Pop()

			if t == "&&" {
				eval.Push(first && second)
			} else {
				eval.Push(first || second)
			}
		default:
			eval.Push(slices.Contains(tags, t))
		}
	}

	if eval.Empty() {
		logger.Error("extra tokens in result stack")
		return false
	}

	return eval.Pop()
}
