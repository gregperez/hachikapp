// Command checkdsn validates DATABASE_TEST_URL without printing it.
package main

import (
	"fmt"
	"os"

	"github.com/gregperez/hachikapp-backend/internal/test/testdb"
)

func main() {
	if _, err := testdb.RequireURL(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
