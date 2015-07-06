package main

/*
   Go implementation of openssl-vulnkey

   Requirements:
   apt-get install -y openssl-blacklist openssl-blacklist-extra
*/

import (
	"bufio"
	"crypto/rsa"
	"encoding/hex"
	"github.com/deckarep/golang-set"
	"github.com/zmap/zgrab/ztools/x509"
	"log"
	"os"
	"strconv"
	"strings"
)

type OpensslBlacklist struct {
	sets map[int]mapset.Set
}

func NewOpensslBlacklist() (blacklist *OpensslBlacklist) {
	blacklist = &OpensslBlacklist{
		sets: make(map[int]mapset.Set),
	}

	sizes := []int{512, 1024, 2048, 4096}

	for _, size := range sizes {
		set := mapset.NewThreadUnsafeSet()
		path := "/usr/share/openssl-blacklist/blacklist.RSA-" + strconv.Itoa(size)

		log.Println("Importing blacklist", path)

		file, err := os.Open(path)
		if err != nil {
			log.Print(err)
			log.Fatal("Please install required packages: apt-get install -y openssl-blacklist openssl-blacklist-extra")
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "#") {
				if bytes, err := hex.DecodeString(line); err != nil {
					log.Fatal(err)
				} else {
					set.Add(string(bytes))
				}
			}
		}

		file.Close()
		blacklist.sets[size] = set
	}

	return blacklist
}

func (blacklist *OpensslBlacklist) Contains(cert *x509.Certificate) bool {
	key, ok := cert.PublicKey.(*rsa.PublicKey)

	if !ok {
		return false
	}

	set, exist := blacklist.sets[key.N.BitLen()]

	if !exist {
		return false
	}

	modulus := []byte(strings.ToUpper(hex.EncodeToString(key.N.Bytes())))
	bytes := append(append([]byte("Modulus="), modulus...), '\n')
	fingerprint := []byte(x509.SHA1Fingerprint(bytes))

	return set.Contains(string(fingerprint[10:]))
}
