%{
package selectorconfigparser
%}

%union {
}

%token COLUMN_IDENTIFIER
%token OPERATOR
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
      | LEFT_PARENTHESIS clause RIGHT_PARENTHESIS
;

value:  VALUE_PLACEHOLDER
      | LEFT_PARENTHESIS value RIGHT_PARENTHESIS
;

%%

