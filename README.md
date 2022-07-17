# `serial-flash` - Tool for interacting with Pico bootloaders

This is a command-line tool for uploading code to Raspberry Pi Picos, either
running:
* [`picowota`](https://github.com/usedbytes/picowota) for WiFi upload to a Pico W
* [`rp2040_serial_bootloader`](https://github.com/usedbytes/rp2040-serial-bootloader), for upload
  over UART

It's a `go` program, so can be installed like any other.

First install `go`: https://go.dev/doc/install

Then install `serial-flash`:

```
go install github.com/usedbytes/serial-flash@latest
```

Then (assuming your `go` binary install location is on your `$PATH`, see you can
run `serial-flash`:

```
# picowota
serial-flash tcp:192.168.1.123 app.elf

# rp2040_serial_bootloader
serial-flash /dev/ttyUSB0 app.elf
```

You can also build it without installing:

```
git clone https://github.com/usedbytes/serial-flash
cd serial-flash
go get -v .
go build .
./serial-flash tcp:192.168.1.123 app.elf
```
