# Nome do binário gerado
BINARY_NAME=ninjaprice

# O comando default (executado quando digitas apenas 'make')
all: build run

# Faz o build do projeto (compila o binário)
build:
	@echo "==> A compilar o NinjaPrice..."
	go build -o $(BINARY_NAME) .

# Executa o binário gerado
run:
	@echo "==> A iniciar o NinjaPrice..."
	./$(BINARY_NAME)

# Limpa o binário gerado anteriormente
clean:
	@echo "==> A remover ficheiros de build antigos..."
	rm -f $(BINARY_NAME)

# Atalho utilitário para formatar o código e limpar as dependências
tidy:
	@echo "==> A organizar dependências e formatar código..."
	go fmt ./...
	go mod tidy