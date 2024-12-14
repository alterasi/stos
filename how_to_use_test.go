package main

import (
	"github.com/alterasi/stos/example/mapper"
	"log"
	"testing"
)

func TestGenerateMapperStos(t *testing.T) {
	err := NewMapperGenerator((*mapper.MapperUser)(nil))
	if err != nil {
		log.Fatalf("Error generating code: %v", err)
	}
}
