# qrcode is a starpkg module the Go shell chose to wire in (see main.go). The
# shell's whole dependency tree is starbox + qrcode — nothing else.
qr = qrcode.encode("https://github.com/1set/starcli")
print("QR for the starcli repo (%d×%d modules):" % (qr.size, qr.size))
print(qr.ascii())
