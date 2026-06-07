# High-Level Domain Dialects

## Goal

Leia should not stop at low-level protocol tags. `urlquery`, `headers`,
`multipart`, and `mailaddr` are useful building blocks, but users need
high-level dialects that directly complete tasks.

## Layering

Low-level:

```leia
urlquery`a=1&b=2`
headers`content-type: text/plain`
multipart`...`
```

High-level:

```leia
web { ... }
api { ... }
mail { ... }
db { ... }
workflow { ... }
test { ... }
```

High-level dialects may use low-level format/protocol dialects internally.

## Web

```leia
page := web {
    fetch: `https://example.com/articles/${id}`
    decode: "html"
    select: "main article"
}
```

Requirements:

- fetch pages;
- download files;
- scrape HTML;
- submit forms;
- serve routes later;
- expose status, headers, body, parsed content.

## API

```leia
result := api {
    method: "GET"
    url: `https://api.example.com/users/${id}`
    headers: {
        Authorization: `Bearer ${token}`
    }
    decode: "json"
    retry: 3
}
```

Requirements:

- common REST calls;
- JSON request/response support;
- retry;
- pagination;
- auth headers;
- SSE/streaming where needed.

## Mail

```leia
mail {
    to: ["team@example.com"]
    subject: `Release ${version}`
    body: `Release passed.`
    attachments: [path`report.html`]
}
```

Requirements:

- compose email;
- send email through configured provider;
- parse incoming email later;
- reuse address/MIME/multipart primitives.

## Database

```leia
rows := db {
    source: "main"
    query: sql`select * from users where id = ${id}`
}
```

Requirements:

- connect;
- query;
- execute;
- transaction;
- migration;
- seed/test data;
- scan rows into tables.

## Workflow

```leia
workflow {
    step: "test"
    run: sh!`go test ./...`
}
```

Requirements:

- named steps;
- dependencies;
- shell commands;
- AI/evaluate steps;
- reports;
- optional approval.

## Test

```leia
test {
    name: "agent remembers user"
    run: fn() {
        result := support("hello")
        assert(result.text != "")
    }
}
```

Requirements:

- unit tests;
- golden/snapshot;
- benchmarks;
- evaluation;
- report output.

## Non-Goals

- Do not expose only protocol primitives and call that a web/mail/api feature.
- Do not duplicate low-level codec behavior inside high-level dialects.
