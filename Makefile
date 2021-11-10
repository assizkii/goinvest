.PHONY: proto
#generate proto files and gql scheme from pb
proto:
	@echo 'generate proto'
	rm -rf ./gen/proto && buf generate
#generate proto files and graphQL scheme
gql:
	@echo 'generate gql'
	rm -rf ./gen/gql && cd ./api/gql && go run -mod=mod github.com/99designs/gqlgen --verbose --config gqlgen.yml

gen-scheme: proto gql
