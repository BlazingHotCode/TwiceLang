package main

import (
	"fmt"
	"twice/internal/lexer"
	"twice/internal/token"
)

func main() {
	input := `let five = 5;
let ten = 10;

let add = fn(x, y) {
	x + y;
};

let result = add(five, ten);
!-/*5;
5 < 10 > 5;

if (5 < 10) {
	return true;
} else {
	return false;
}

10 == 10;
10 != 9;
`

	l := lexer.New(input)

	// Keep getting tokens until we hit EOF
	for {
		tok := l.NextToken()
		fmt.Printf("%+v\n", tok)
		
		if tok.Type == token.EOF {
			break
		}
	}
}
