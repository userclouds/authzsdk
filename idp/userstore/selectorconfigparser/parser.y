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
%token ANY
%token CONJUNCTION
%token LEFT_PARENTHESIS
%token RIGHT_PARENTHESIS
%token UNKNOWN

%%
clause: term
      | term CONJUNCTION clause
;

term:   COLUMN_IDENTIFIER OPERATOR value
      | COLUMN_IDENTIFIER OPERATOR ANY value
      | COLUMN_IDENTIFIER null_check
      | LEFT_PARENTHESIS clause RIGHT_PARENTHESIS
;

value:  VALUE_PLACEHOLDER
      | LEFT_PARENTHESIS value RIGHT_PARENTHESIS
;

null_check: IS NULL
      | IS NOT NULL
;

%%

