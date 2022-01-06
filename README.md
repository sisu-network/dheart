This is a repo that contains TSS engine of Sisu network.

# Run dheart locally

Generate dheart config file

```
go run core/config/gen-localhost/main.go
```

Create environment file .env and add this
```
HOME_DIR=/Users/yourusername/.sisu/dheart
AES_KEY_HEX=c787ef22ade5afc8a5e22041c17869e7e4714190d88ecec0a84e241c9431add0
```

Install all modules.
```
go mod tidy
```

Build and run dheart

```
go build && ./dheart
```
