; Leia tree-sitter highlights.

[
  "case"
  "default"
  "defer"
  "else"
  "elseif"
  "for"
  "go"
  "goto"
  "if"
  "range"
  "return"
  "select"
] @keyword.control

[
  "const"
  "func"
  "import"
] @keyword.function

[
  "as"
] @keyword

[
  "true"
  "false"
] @boolean


(comment) @comment
(string) @string
(number) @number
(duration) @number
(dense_type) @type.builtin

(function_declaration
  name: (identifier) @function)

(tagged_string_expression
  tag: (identifier) @tag)

(tagged_string_expression
  tag: (shell_tag) @tag)

(tagged_block_expression
  tag: (identifier) @tag)

(dialect_bang) @operator

(import_declaration
  alias: (identifier) @namespace)

(parameter) @variable.parameter
(vararg_parameter) @variable.parameter

(call_expression
  function: (expression (identifier) @function.call))

(method_call_expression
  method: (identifier) @function.method.call)

(field_expression
  field: (identifier) @property)

(config_field
  key: (identifier) @property)

(table_field
  key: (identifier) @property)

[
  "+"
  "-"
  "*"
  "/"
  "%"
  "**"
  "=="
  "!="
  "<"
  "<="
  ">"
  ">="
  "&&"
  "||"
  "!"
  "&"
  "|"
  "^"
  "&^"
  "<<"
  ">>"
  ".."
  "<-"
  "="
  ":="
  "+="
  "-="
  "*="
  "/="
] @operator

[
  "("
  ")"
  "["
  "]"
  "{"
  "}"
] @punctuation.bracket

[
  ","
  ";"
  ":"
  "."
] @punctuation.delimiter
