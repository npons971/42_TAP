.PHONY: install-go clean

install-go: go

install_files/go1.27.1.linux-amd64.tar.gz:
	mkdir -p install_files
	curl -fL --retry 3 -o $@.tmp https://go.dev/dl/go1.27.1.linux-amd64.tar.gz && \
	  printf '%s  %s\n' 63d339f0da5ab53635a56f2490a7984dfe12dfcff22ad749f63edaf590168445 $@.tmp | sha256sum -c - && \
	  mv $@.tmp $@

go: install_files/go1.27.1.linux-amd64.tar.gz
	mkdir -p .go-sdk
	tar -xzf install_files/go1.27.1.linux-amd64.tar.gz -C .go-sdk
	ln -s .go-sdk/go/bin/go go

clean:
	rm -rf .go-sdk go
	rm -f install_files/go1.27.1.linux-amd64.tar.gz
