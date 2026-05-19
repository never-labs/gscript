print("case:constructs_label_goto_control")

i := 0
sum := 0
loop:
i++
if i > 6 {
	goto done
}
if i == 2 {
	goto loop
}
sum += i
goto loop

done:;
print("sum", sum)

escaped := "no"
for {
	escaped = "yes"
	goto after_loop
}
after_loop:;
print("escaped", escaped)

func collect(n) {
	out := ""
again:
	if n <= 0 {
		goto finish
	}
	out = out .. tostring(n)
	n--
	goto again
finish:
	return out
}

print("collect", collect(4))
