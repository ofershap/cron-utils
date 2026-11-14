// cron-utils describes cron expressions in English and previews fire times.
//
//	cron-utils "0 9 * * 1-5"
//	cron-utils "0 9 * * FRI#3" --next 3
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	cronutils "github.com/ofershap/cron-utils"
)

func main() {
	next := flag.Int("next", 0, "also print the next N fire times")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: cron-utils [--next N] \"<cron expression>\"\n")
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	s, err := cronutils.Parse(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(s.Describe())
	for _, t := range s.NextN(time.Now(), *next) {
		fmt.Println(t.Format("Mon Jan 2, 2006 15:04"))
	}
}
