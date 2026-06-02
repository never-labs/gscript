; Leia tree-sitter highlights for Helix.
; Keep in sync with tools/tree-sitter-leia/queries/highlights.scm.

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
] @constant.builtin.boolean


(comment) @comment
(string) @string
(number) @constant.numeric.integer
(duration) @constant.numeric.integer
(dense_type) @type.builtin

(function_declaration name: (identifier) @function)
(tool_declaration name: (identifier) @function)
(agent_declaration name: (identifier) @type)
(evaluate_block name: (string) @string.special)
(import_declaration alias: (identifier) @namespace)
(parameter) @variable.parameter
(vararg_parameter) @variable.parameter
(call_expression function: (identifier) @function)
(method_call_expression method: (identifier) @function.method)
(field_expression field: (identifier) @variable.other.member)
(config_field key: (identifier) @variable.other.member)
(table_field key: (identifier) @variable.other.member)
(message_field key: (identifier) @variable.other.member)

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
