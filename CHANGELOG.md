# Changelog

## 1.4.0 - 28-06-2024

- Deprecate v1.0.0 and v1.1.0

## 1.3.0 - 28-06-2024

- Retry getting access token on EOF type network errors which occur when connection is lost.
- Breaking change: RetryNetworkErrors option for jsonclient now takes a boolean, and the option is on by default

## 1.2.0 - 09-04-2024

- Update userstore sample to exercise partial update columns
- Add methods for creating, retrieving, updating, and deleting ColumnDataTypes
- Add DataType field to Column that refers to a ColumnDataType
- Add InputDataType and OutputDataType fields to Transformer that refer to ColumnDataTypes
- Update userstore sample to interact with ColumnDataTypes
- Breaking change: Add additional boolean parameter to ListAccessors an ListMutators for requesting all versions

## 1.1.0 - 20-03-2024

- Breaking change: idp/userstore/ColumnField parameter "Optional" has been changed to "Required", with fields not required by default
- Add InputConstraints and OutputConstraints parameters of type idp/userstore/ColumnConstraints to idp/policy/Transformer
- Add pagination support for chained logical filter queries (query,logical_operator,query,logical_operator,query...)

## 1.0.0 - 31-01-2024

- Breaking change: Return ErrXXXNotFound error when getting a HTTP 404 from authz endpoints
- Breaking change: Move prefix argument for NewRedisClientCacheProvider to be optional KeyPrefixRedis(prefix) instead of required
- Add validation of non empty pointer strings
- Add column constraints implementation
- Adding "ID" as an optional field for user creation
