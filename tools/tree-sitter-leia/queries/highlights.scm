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
  "agent"
  "budget"
  "evaluate"
  "flow"
  "messages"
  "models"
  "tool"
  "turn"
] @keyword

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

(tool_declaration
  name: (identifier) @function)

(agent_declaration
  name: (identifier) @type)

(evaluate_block
  name: (string) @string.special)

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

(message_field
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
