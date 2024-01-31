# Changelog

## 1.0.0 - UNPUBLISHED

- Breaking change: Return ErrXXXNotFound error when getting a HTTP 404 from authz endpoints
- Breaking change: Move prefix argument for NewRedisClientCacheProvider to be optional KeyPrefixRedis(prefix) instead of required
- Add validation of non empty pointer strings
- Add column constraints implementation
- Adding "ID" as an optional field for user creation