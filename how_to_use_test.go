package main

import (
	"github.com/alterasi/stos/example/mapper"
	"log"
	"testing"
)

func TestGenerateMapperStos(t *testing.T) {
	mapa, err := NewMapperGenerator((*mapper.MapperUser)(nil))
	if err != nil {
		log.Fatalf("Error generating code: %v", err)
	}

	if err = mapa.WriteToFile(); err != nil {
		log.Fatalf("Error generating code: %v", err)
	}

	return
}
