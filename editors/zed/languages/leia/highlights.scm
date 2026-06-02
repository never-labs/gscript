; Leia tree-sitter highlights for Zed.
; Keep in sync with tools/tree-sitter-leia/queries/highlights.scm.

[
  "break"
  "case"
  "continue"
  "default"
  "defer"
  "else"
  "elseif"
  "for"
  "go"
  "goto"
  "if"
  "in"
  "range"
  "return"
  "select"
] @keyword

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
  "as"
  "var"
] @keyword

[
  "true"
  "false"
] @boolean

"nil" @constant
"..." @punctuation

(comment) @comment
(string) @string
(number) @number
(duration) @number
(dense_type) @type

(function_declaration name: (identifier) @function)
(tool_declaration name: (identifier) @function)
(agent_declaration name: (identifier) @type)
(evaluate_block name: (string) @string)
(import_declaration alias: (identifier) @namespace)
(parameter) @variable
(vararg_parameter) @variable
(call_expression function: (identifier) @function)
(method_call_expression method: (identifier) @function)
(field_expression field: (identifier) @property)
(config_field key: (identifier) @property)
(table_field key: (identifier) @property)
(message_field key: (identifier) @property)

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
  "%="
] @operator

[
  "("
  ")"
  "["
  "]"
  "{"
  "}"
] @punctuation

[
  ","
  ";"
  ":"
  "."
] @punctuation
