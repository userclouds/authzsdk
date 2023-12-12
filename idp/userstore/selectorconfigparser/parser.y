%{
package selectorconfigparser
%}

%union {
}

%token COLUMN_IDENTIFIER
%token OPERATOR
%token IS
%token NOT
%token NULL
%token VALUE_PLACEHOLDER
%token INT_VALUE
%token INT_VALUE_LIST
%token QUOTED_VALUE
%token QUOTED_VALUE_LIST
%token ANY
%token CONJUNCTION
%token LEFT_PARENTHESIS
%token RIGHT_PARENTHESIS
%token UNKNOWN

%%
clause: term
      | term CONJUNCTION clause
;

term:   COLUMN_IDENTIFIER OPERATOR ANY array_value
      | COLUMN_IDENTIFIER OPERATOR value
      | COLUMN_IDENTIFIER null_check
      | LEFT_PARENTHESIS clause RIGHT_PARENTHESIS
;

value:  VALUE_PLACEHOLDER
      | INT_VALUE
      | QUOTED_VALUE
      | LEFT_PARENTHESIS value RIGHT_PARENTHESIS
;

array_value:  VALUE_PLACEHOLDER
            | INT_VALUE_LIST
            | QUOTED_VALUE_LIST
            | LEFT_PARENTHESIS array_value RIGHT_PARENTHESIS
;

null_check: IS NULL
      | IS NOT NULL
;

%%

