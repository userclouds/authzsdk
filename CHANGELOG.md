# Changelog

## UNPUBLISHED

- Breaking change: idp/userstore/ColumnField parameter "Optional" has been changed to "Required", with fields not required by default
- Add InputConstraints and OutputConstraints parameters of type idp/userstore/ColumnConstraints to idp/policy/Transformer

## 1.0.0 - 31-01-2024

- Breaking change: Return ErrXXXNotFound error when getting a HTTP 404 from authz endpoints
- Breaking change: Move prefix argument for NewRedisClientCacheProvider to be optional KeyPrefixRedis(prefix) instead of required
- Add validation of non empty pointer strings
- Add column constraints implementation
- Adding "ID" as an optional field for user creation
