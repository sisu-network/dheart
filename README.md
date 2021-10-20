This is a repo that contains TSS engine of Sisu network.

# Run dheart locally

Create a .env file with content similar to .env.dev. Set the HOME dir of dheart

```
# Env file for local dev. Set the HOME Dir to be the home dir of dheart. For example
# /User/ubuntu/.sisu/dheart
USE_ON_MEMORY=true
HOME_DIR= [SET_DHEART_DIR_HERE]
SISU_SERVER_URL=http://localhost:25456
DB_HOST=localhost
DB_PORT=3306
DB_USERNAME=root
DB_PASSWORD=password
DB_SCHEMA=dheart
DB_MIGRATION_PATH=file://db/migrations/
AES_KEY_HEX=c787ef22ade5afc8a5e22041c17869e7e4714190d88ecec0a84e241c9431add0
```

Build and run dheart

```
go build && ./dheart
```
