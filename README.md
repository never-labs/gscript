# Leia

Leia is an efficient, embeddable scripting language for Go, combining a LuaJIT-class execution model, q-style high-throughput in-memory columnar analytics, and first-class extensible domain dialects.

```go
a := [1,2,3,4,5,6,7,8,6]
x := q`sum ${a}`

answer, err := turn {
    model: "mock-fast"
    messages: [
        prompt { role: "user", text: "Explain why ${x} matters." }
    ]
}

if err == nil {
    print(answer.text)
} else {
    print(x)
}
```
