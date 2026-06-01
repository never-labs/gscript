# Tables And Metatables

Tables are mutable identity-bearing maps. Keys and values are runtime values.
Implementations may use optimized array, record, or typed layouts when this
does not change observable behavior.

Table constructors create fresh table identities. List-style fields are assigned
using 1-based integer sequence keys. Keyed fields assign the specified key.

Raw helpers bypass metamethods by contract. Non-raw operations may consult
metatables when the runtime supports the corresponding metamethod.

Stable metatable behavior includes indexing, new indexing, call, arithmetic,
comparison, concatenation, and length behavior where covered by conformance
tests and the feature matrix. Exact Lua debug-slot protocols, binary chunks,
and finalizer behavior are not stable Leia promises unless specified
separately.

Sequence length on sparse tables follows Leia runtime behavior. Programs that
depend on sparse length edge cases should pin that behavior with tests.
