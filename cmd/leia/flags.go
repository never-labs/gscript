package main

import "flag"

func parseCLIFlags(fs *flag.FlagSet, args []string) (int, bool) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0, true
		}
		return 2, true
	}
	return 0, false
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func resolveVMJITFlags(fs *flag.FlagSet, useVM, useJIT *bool) {
	vmExplicit := flagWasSet(fs, "vm")
	jitExplicit := flagWasSet(fs, "jit")

	// -vm without -jit means "VM only, no JIT".
	if vmExplicit && !jitExplicit {
		*useJIT = false
	}
	if *useJIT {
		*useVM = true
	}
}
