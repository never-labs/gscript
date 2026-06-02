const PREC = {
  assign: -1,
  or: 1,
  and: 2,
  compare: 3,
  bit_or: 4,
  bit_xor: 5,
  bit_and: 6,
  shift: 7,
  concat: 8,
  additive: 9,
  multiplicative: 10,
  unary: 11,
  power: 12,
  call: 13,
};

module.exports = grammar({
  name: "leia",

  extras: $ => [
    /[\s\uFEFF\u2060\u200B]/,
    $.comment,
  ],

  word: $ => $.identifier,

  conflicts: $ => [
    [$.expression_statement, $.table_field],
    [$.expression_list, $.table_field],
  ],

  rules: {
    source_file: $ => repeat(choice($.separator, $.statement)),

    separator: _ => ";",

    comment: _ => token(choice(
      seq("//", /.*/),
      seq("/*", /[^*]*\*+([^/*][^*]*\*+)*/, "/"),
    )),

    statement: $ => choice(
      $.function_declaration,
      $.import_declaration,
      $.tool_declaration,
      $.agent_defaults_declaration,
      $.agent_declaration,
      $.models_declaration,
      $.budget_statement,
      $.evaluate_block,
      $.if_statement,
      $.for_statement,
      $.select_statement,
      $.return_statement,
      $.break_statement,
      $.continue_statement,
      $.goto_statement,
      $.label_statement,
      $.go_statement,
      $.defer_statement,
      $.const_declaration,
      $.simple_statement,
    ),

    block: $ => prec(1, seq("{", repeat(choice($.separator, $.statement)), "}")),

    function_declaration: $ => seq(
      "func",
      field("name", $.identifier),
      field("parameters", $.parameter_list),
      field("body", $.block),
    ),

    import_declaration: $ => seq(
      "import",
      field("path", $.string),
      "as",
      field("alias", $.identifier),
    ),

    tool_declaration: $ => seq(
      "tool",
      field("name", $.identifier),
      field("parameters", $.parameter_list),
      field("body", $.block),
    ),

    agent_defaults_declaration: $ => seq(
      "agent",
      "defaults",
      field("config", $.config_block),
    ),

    agent_declaration: $ => seq(
      "agent",
      field("name", $.identifier),
      optional(field("parameters", $.parameter_list)),
      field("config", $.config_block),
      optional(field("flow", $.flow_block)),
    ),

    models_declaration: $ => seq("models", field("config", $.config_block)),

    budget_statement: $ => seq(
      "budget",
      field("config", $.config_block),
      field("body", $.block),
    ),

    evaluate_block: $ => seq(
      "evaluate",
      field("name", $.string),
      field("body", $.block),
    ),

    flow_block: $ => seq("flow", $.block),

    parameter_list: $ => seq(
      "(",
      optional(seq(
        choice($.parameter, $.vararg_parameter),
        repeat(seq(",", choice($.parameter, $.vararg_parameter))),
        optional(","),
      )),
      ")",
    ),

    parameter: $ => $.identifier,
    vararg_parameter: $ => choice("...", seq($.identifier, "...")),

    if_statement: $ => seq(
      "if",
      field("condition", $.expression),
      field("consequence", $.block),
      repeat($.elseif_clause),
      optional($.else_clause),
    ),

    elseif_clause: $ => seq(
      "elseif",
      field("condition", $.expression),
      field("consequence", $.block),
    ),

    else_clause: $ => seq("else", field("alternative", $.block)),

    for_statement: $ => choice(
      seq("for", field("body", $.block)),
      seq("for", field("condition", $.expression), field("body", $.block)),
      seq(
        "for",
        field("initializer", $.simple_statement),
        ";",
        field("condition", $.expression),
        ";",
        field("update", $.simple_statement),
        field("body", $.block),
      ),
      prec(1, seq(
        "for",
        field("key", $.identifier),
        optional(seq(",", field("value", $.identifier))),
        ":=",
        "range",
        field("iterable", $.expression),
        field("body", $.block),
      )),
    ),

    select_statement: $ => seq(
      "select",
      "{",
      repeat($.select_case),
      "}",
    ),

    select_case: $ => choice(
      seq("case", choice($.select_receive_clause, $.select_send_clause), ":", repeat(choice($.separator, $.statement))),
      seq("default", ":", repeat(choice($.separator, $.statement))),
    ),

    select_receive_clause: $ => choice(
      seq("<-", field("channel", $.expression)),
      seq(field("name", $.identifier), ":=", "<-", field("channel", $.expression)),
      seq(field("name", $.identifier), ",", field("ok", $.identifier), ":=", "<-", field("channel", $.expression)),
    ),

    select_send_clause: $ => seq(
      field("channel", $.expression),
      "<-",
      optional(field("value", $.expression)),
    ),

    return_statement: $ => prec.right(seq("return", optional($.expression_list))),
    break_statement: _ => "break",
    continue_statement: _ => "continue",
    goto_statement: $ => seq("goto", field("label", $.identifier)),
    label_statement: $ => seq("::", field("label", $.identifier), "::"),
    go_statement: $ => seq("go", field("call", $.expression)),
    defer_statement: $ => seq("defer", field("call", $.expression)),

    const_declaration: $ => seq(
      "const",
      field("name", $.identifier),
      choice("=", ":="),
      field("value", $.expression),
    ),

    simple_statement: $ => choice(
      $.assignment_statement,
      $.compound_assignment_statement,
      $.increment_statement,
      $.send_statement,
      $.call_statement,
      $.expression_statement,
    ),

    assignment_statement: $ => prec.right(PREC.assign, seq(
      field("left", $.expression_list),
      choice("=", ":="),
      field("right", $.expression_list),
    )),

    compound_assignment_statement: $ => seq(
      field("left", $.expression),
      choice("+=", "-=", "*=", "/="),
      field("right", $.expression),
    ),

    increment_statement: $ => seq(field("argument", $.expression), choice("++", "--")),
    send_statement: $ => prec(1, seq(field("channel", $.expression), "<-", field("value", $.expression))),
    call_statement: $ => prec(1, $.call_expression),
    expression_statement: $ => $.expression,

    expression_list: $ => seq($.expression, repeat(seq(",", $.expression))),

    expression: $ => choice(
      $.identifier,
      $.number,
      $.duration,
      $.string,
      $.boolean,
      $.nil,
      $.vararg_expression,
      $.parenthesized_expression,
      $.function_literal,
      $.agent_literal,
      $.turn_expression,
      $.messages_expression,
      $.table_literal,
      $.list_literal,
      $.dense_literal,
      $.unary_expression,
      $.binary_expression,
      $.receive_expression,
      $.field_expression,
      $.index_expression,
      $.call_expression,
      $.method_call_expression,
    ),

    parenthesized_expression: $ => seq("(", $.expression, ")"),

    function_literal: $ => seq("func", $.parameter_list, $.block),

    agent_literal: $ => seq(
      "agent",
      optional($.parameter_list),
      field("config", $.config_block),
      optional(field("flow", $.flow_block)),
    ),

    turn_expression: $ => seq("turn", field("config", $.config_block)),
    messages_expression: $ => seq("messages", $.messages_block),

    messages_block: $ => seq(
      "{",
      optional(seq($.message_field, repeat(seq($.field_separator, $.message_field)), optional($.field_separator))),
      "}",
    ),

    message_field: $ => choice(
      prec(1, seq(field("key", choice($.identifier, $.string)), ":", field("value", $.expression))),
      prec(1, seq("[", field("key", $.expression), "]", ":", field("value", $.expression))),
      field("value", $.expression),
    ),

    config_block: $ => seq(
      "{",
      optional(seq($.config_field, repeat(seq($.field_separator, $.config_field)), optional($.field_separator))),
      "}",
    ),

    config_field: $ => choice(
      prec(1, seq(field("key", choice($.identifier, $.string)), ":", field("value", $.expression))),
      prec(1, seq("[", field("key", $.expression), "]", ":", field("value", $.expression))),
    ),

    table_literal: $ => seq(
      "{",
      optional(seq($.table_field, repeat(seq($.field_separator, $.table_field)), optional($.field_separator))),
      "}",
    ),

    table_field: $ => choice(
      prec(1, seq(field("key", $.identifier), ":", field("value", $.expression))),
      prec(1, seq("[", field("key", $.expression), "]", ":", field("value", $.expression))),
      field("value", $.expression),
    ),

    list_literal: $ => seq(
      "[",
      optional(seq($.expression, repeat(seq($.field_separator, $.expression)), optional($.field_separator))),
      "]",
    ),

    dense_literal: $ => prec(1, seq(
      "[",
      optional(field("length", $.number)),
      "]",
      field("type", $.dense_type),
      "{",
      optional(seq($.expression, repeat(seq($.field_separator, $.expression)), optional($.field_separator))),
      "}",
    )),

    dense_type: _ => choice("i32", "i64", "f32", "f64", "bool"),
    field_separator: _ => choice(",", ";"),

    unary_expression: $ => prec(PREC.unary, seq(
      field("operator", choice("!", "-", "#", "^")),
      field("argument", $.expression),
    )),

    receive_expression: $ => prec(PREC.unary, seq("<-", field("channel", $.expression))),

    binary_expression: $ => choice(
      ...[
        ["||", PREC.or],
        ["&&", PREC.and],
        ["==", PREC.compare],
        ["!=", PREC.compare],
        ["<", PREC.compare],
        ["<=", PREC.compare],
        [">", PREC.compare],
        [">=", PREC.compare],
        ["|", PREC.bit_or],
        ["^", PREC.bit_xor],
        ["&", PREC.bit_and],
        ["&^", PREC.bit_and],
        ["<<", PREC.shift],
        [">>", PREC.shift],
        ["..", PREC.concat],
        ["+", PREC.additive],
        ["-", PREC.additive],
        ["*", PREC.multiplicative],
        ["/", PREC.multiplicative],
        ["%", PREC.multiplicative],
      ].map(([operator, precedence]) => prec.left(precedence, seq(
        field("left", $.expression),
        field("operator", operator),
        field("right", $.expression),
      ))),
      prec.right(PREC.power, seq(
        field("left", $.expression),
        field("operator", "**"),
        field("right", $.expression),
      )),
    ),

    field_expression: $ => prec.left(PREC.call, seq(
      field("object", $.expression),
      ".",
      field("field", $.identifier),
    )),

    index_expression: $ => prec.left(PREC.call, seq(
      field("object", $.expression),
      "[",
      field("index", $.expression),
      "]",
    )),

    call_expression: $ => prec.left(PREC.call, seq(
      field("function", $.expression),
      field("arguments", $.argument_list),
    )),

    method_call_expression: $ => prec.left(PREC.call, seq(
      field("object", $.expression),
      ":",
      field("method", $.identifier),
      field("arguments", $.argument_list),
    )),

    argument_list: $ => seq(
      "(",
      optional(seq($.expression, repeat(seq(",", $.expression)), optional(","))),
      ")",
    ),

    boolean: _ => choice("true", "false"),
    nil: _ => "nil",
    vararg_expression: _ => "...",
    duration: $ => token(seq(/[0-9][0-9_]*/, choice("ns", "us", "µs", "ms", "s", "m", "h"))),
    number: _ => token(choice(
      /0[xX][0-9a-fA-F_]+/,
      /0[bB][01_]+/,
      /0[oO][0-7_]+/,
      /[0-9][0-9_]*\.[0-9][0-9_]*([eE][+-]?[0-9][0-9_]*)?/,
      /[0-9][0-9_]*[eE][+-]?[0-9][0-9_]*/,
      /[0-9][0-9_]*/,
    )),
    string: _ => token(choice(
      seq('"', repeat(choice(/[^"\\\n]/, /\\./)), '"'),
      seq("`", repeat(/[^`]/), "`"),
    )),
    identifier: _ => /[A-Za-z_][A-Za-z0-9_]*/,
  },
});
