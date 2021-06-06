# go_monkey

I got my hands dirty while reading the book "Writing an interpreter in Go".

And then I'm done with "Writing a Compiler in Go".

* Lexer
* Parser that transforms source codes(tokens lexed by Lexer) to AST structure. Here, Top down operator precedence parser (a.k.a. Pratt parser) is picked up and implemented. 
* AST (Abstract Syntax Tree)
* Interpreter that traverses the AST and executes on the fly.
* Compiler that traverses the AST and generates the corresponding bytecode containing instructions, each instruction is made up of a opcode and one or two operands, or no operands at all. Jump instruction to implement conditionals.
* Stack based virtual machine that executes the bytecode and do stack arithmetic and so forth using a stack. Keeping track of variable bindings, global or local ones.
* Closure, free variable. Recursive funcition call, function that calls itself inside it.
