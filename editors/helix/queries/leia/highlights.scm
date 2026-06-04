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
] @keyword.function

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
  tag: (identifier) @tag.dialect
  body: (string) @string.special.dialect)

(tagged_string_expression
  tag: (shell_tag) @tag.shell
  body: (string) @string.special.shell)

(tagged_block_expression
  tag: (identifier) @tag.dialect)

(tagged_string_expression
  bang: (dialect_bang) @operator.raw.dialect)

(tagged_block_expression
  bang: (dialect_bang) @operator.raw.dialect)

(import_declaration
  "import" @keyword.control.import)

(import_spec
  path: (string) @string.special.import
  "as" @keyword.control.import.as
  alias: (identifier) @namespace.import)

(import_spec
  alias: (identifier) @namespace.import
  path: (string) @string.special.import)

(import_spec
  path: (string) @string.special.import)

(unary_expression
  operator: "!" @operator)

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
