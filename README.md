# GScript

Go-syntax scripting language with a Lua-like runtime and ARM64 JIT.

## Test

```bash
go test ./... -count=1 -p 1 -timeout=600s
python3 benchmarks/timing_compare.py --all --runs 7 --warmup 2 --timeout 900 --time-source script
```

## Performance

Concurrent full benchmark run, sorted by `Cur/LJ`.
`suite/fib` and `suite/fib_recursive` use high-repeat extrapolation because the normal run is below timer resolution.

```text
Benchmark                                     Current       HEAD     LuaJIT  Cur/HEAD   Cur/LJ       CV   Exits
-----------------------------------------------------------------------------------------------------------------
official/defer_protected_hot                0.315000s  0.315000s  0.315000s     1.00x    1.00x    0.00%       0
official/nextvar_table_hot                  0.025000s  0.025000s  0.025000s     1.00x    1.00x    1.20%       0
extended/actors_dispatch_mutation           0.407000s  0.402000s  0.486000s     1.01x    0.84x    0.63%       0
suite/method_dispatch                       0.057500s  0.054000s  0.070000s     1.06x    0.82x    3.80%       0
suite/table_array_access                    0.045000s  0.044500s  0.057000s     1.01x    0.79x    1.09%       2
extended/groupby_nested_agg                 0.367000s  0.366000s  0.477000s     1.00x    0.77x    1.21%      59
suite/sum_primes                            0.018750s  0.018250s  0.024500s     1.03x    0.77x    1.86%       0
suite/matmul                                0.016250s  0.016750s  0.021500s     0.97x    0.76x    2.31%       1
variants/sort_mixed_numeric                 0.027000s  0.027000s  0.036000s     1.00x    0.75x    1.83%       4
suite/math_intensive                        0.042500s  0.043500s  0.058500s     0.98x    0.73x    2.10%       0
official/strings_patterns_hot               0.085000s  0.085000s  0.119000s     1.00x    0.71x    0.81%       0
suite/matmul_dense_tb                       0.017000s  0.016500s  0.024500s     1.03x    0.69x    0.72%       2
suite/table_field_access                    0.012625s  0.012250s  0.018750s     1.03x    0.67x    2.12%       0
official/events_metamethod_hot              0.007875s  0.008000s  0.013000s     0.98x    0.61x    0.84%       0
extended/producer_consumer_pipeline         0.026500s  0.026750s  0.045000s     0.99x    0.59x    0.85%       0
suite/fannkuch                              0.011000s  0.011250s  0.019250s     0.98x    0.57x    0.00%       0
extended/json_table_walk                    0.036000s  0.036000s  0.070000s     1.00x    0.51x    3.35%      20
suite/sort                                  0.009875s  0.010000s  0.019250s     0.99x    0.51x    2.44%       1
extended/mixed_inventory_sim                0.067000s  0.067000s  0.132000s     1.00x    0.51x    2.18%       2
suite/nbody                                 0.066000s  0.066000s  0.131000s     1.00x    0.50x    0.74%      12
suite/nbody_dense                           0.016000s  0.016250s  0.032000s     0.98x    0.50x    1.22%       0
variants/closure_accumulator_variant        0.011000s  0.011250s  0.023250s     0.98x    0.47x    2.32%       2
suite/spectral_norm                         0.053500s  0.053500s  0.115000s     1.00x    0.47x    0.50%       0
official/calls_vararg_coroutine_hot         0.002875s  0.002938s  0.006438s     0.98x    0.45x    3.51%       0
suite/coroutine_bench                       0.022500s  0.022500s  0.056000s     1.00x    0.40x    0.84%       0
suite/string_bench                          0.023000s  0.022750s  0.060000s     1.01x    0.38x    0.41%       1
variants/matmul_row_variant                 0.051500s  0.051000s  0.149000s     1.01x    0.35x    0.67%       7
suite/object_creation                       0.015500s  0.015750s  0.046500s     0.98x    0.33x    2.46%      11
official/call_len_pairs_metamethod_hot      0.004000s  0.003938s  0.012750s     1.02x    0.31x    0.84%       0
extended/log_tokenize_format                0.027750s  0.027750s  0.094000s     1.00x    0.30x    1.15%       0
suite/mandelbrot                            0.016500s  0.016250s  0.056500s     1.02x    0.29x    1.20%       0
suite/sieve                                 0.020000s  0.020250s  0.069000s     0.99x    0.29x    0.98%       0
suite/spectral_norm_dense                   0.021000s  0.021250s  0.076000s     0.99x    0.28x    0.58%       0
official/table_sort_proxy_hot               0.017000s  0.017500s  0.070000s     0.97x    0.24x    1.15%       0
suite/closure_bench                         0.010625s  0.010750s  0.044000s     0.99x    0.24x    4.08%       3
variants/ack_nested_shifted                 0.025250s  0.025500s  0.105000s     0.99x    0.24x    1.69%       0
suite/matmul_dense                          0.005938s  0.005938s  0.027500s     1.00x    0.22x    0.83%       2
suite/matmul_dense_split2                   0.006000s  0.006000s  0.028750s     1.00x    0.21x    1.44%       2
suite/ackermann                             0.078000s  0.078000s  0.585000s     1.00x    0.13x    0.68%       0
suite/matmul_dense_unroll2                  0.032000s  0.032000s  0.240000s     1.00x    0.13x    1.08%       2
suite/fibonacci_iterative                   0.002938s  0.002844s  0.025000s     1.03x    0.12x    2.92%       2
official/stdlib_host_hot                    0.003625s  0.003500s  0.100000s     1.04x    0.04x    3.21%       0
official/math_bit_utf8_hot                  0.003000s  0.003000s  0.095000s     1.00x    0.03x    0.00%       0
official/regexp_random_hot                  0.002000s  0.002000s  0.078000s     1.00x    0.03x    0.83%       0
suite/mutual_recursion                      0.103000s  0.103000s  4.057000s     1.00x    0.03x    0.76%       0
suite/binary_trees                          0.002969s  0.002969s  0.164000s     1.00x    0.02x    0.95%       0
suite/fib                                  ~0.000010s ~0.000010s  0.025000s     1.00x  0.0004x       -       0
suite/fib_recursive                        ~0.000019s ~0.000019s  0.333000s     1.00x  0.0001x       -       0
```
