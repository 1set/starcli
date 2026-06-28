# This whole app is Starlark — the Go shell just embeds and runs it.
print("Hello from your embedded Starlark app 👋")

squares = [n * n for n in range(1, 6)]
print("squares 1..5:", squares)
print("√1024 =", math.sqrt(1024))
