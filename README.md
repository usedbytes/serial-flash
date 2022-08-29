# `serial-flash` - Tool for interacting with Pico bootloaders

This is a command-line tool for uploading code to Raspberry Pi Pico
* [`rp2040_serial_bootloader`](https://github.com/mrbeam/RP2040-serial-bootloader), for upload
  over UART

It's a `go` program, so can be installed like any other.

First install `go`: https://go.dev/doc/install

Then install `serial-flash`:

```
go install github.com/mrbeam/RP2040-serial-flash-tool@latest
```

Then (assuming your `go` binary install location is on your `$PATH`, see you can
run `serial-flash`:

```

# rp2040_serial_bootloader
serial-flash /dev/ttyUSB0 app.elf
```

You can also build it without installing:

```
git clone https://github.com/mrbeam/RP2040-serial-flash-tool.git
cd serial-flash
go get -v .
go build .
```
