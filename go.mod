module userclouds.com

go 1.18

require github.com/gofrs/uuid v4.0.0+incompatible

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/joho/godotenv v1.4.0 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/redis/go-redis/v9 v9.0.5 // indirect
)

retract (
  v0.1.0
  v0.2.0
  v0.3.0
  v0.4.0
  v0.5.0
  v0.6.0
  v0.6.1
  v0.6.2
  v0.6.6
  v0.6.7
  v0.7.0
)
