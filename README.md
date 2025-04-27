# DISCON-Wrapper

This project provides a shared library and server application for converting a wind turbine Controller shared library into a network application. This project may be of interest if:

- You have a wind turbine Controller shared library which implements the Bladed API
- The Controller was developed on a 32-bit version of Windows
- You're running simulations in OpenFAST which was compiled in 64-bit on Windows
- You don't have access to the Controller source code to recompile on a new platform
- You have access to a Windows system where you can run an application and the Controller

This project is in alpha stage and requires significant testing. It's unlikely to work on the first try and the configuration is not straightforward.

## What does DISCON-Wrapper do?

This project provides a server, `discon-server.exe`, and a shared library, `discon-client.dll`. The server loads the Controller shared library, typically a Windows DLL, and waits for client connections over a Websocket (TCP connection). The `discon-client.dll` is used in place of the existing Controller in OpenFAST such that when the simulation starts, the client connects to the server, transmits the Controller arguments over the Websocket connection, get's the result, and then supplies the result to OpenFAST. This communication is transparent to OpenFAST so it looks like it's just calling the Controller normally. 

## Why would this be useful?

A major difficulty in running simulations with older controllers is that they were developed when operating systems used 32-bit address spaces; however, most operating systems and software are now 64-bit. It is not possible to load a 32-bit Controller shared library into a 64-bit application like OpenFAST. If you don't have the source code for the Controller, then it's not possible to recompile it for use in a 64-bit application. The alternative is to build OpenFAST with a 32-bit compiler; however, this is becoming more and more difficult as Intel's latest Fortran compiler is not available for 32-bit. The DISCON-Wrapper tools solve this problem by providing a 32-bit server to load the 32-bit Controller shared library and a 64-bit client shared library which can be loaded by a 64-bit simulation application. The server and client communicate via Websocket, eliminating the incompatibility.

## Configuration

There's not a lot to configure, but you're bridging two applications, so things have to match between the client and the server for them to communicate. We'll cover the server first and then the client for the most common use case of a 64-bit OpenFAST connecting to a 32-bit Controller.

### Server

Download `discon-server_386.exe` from [Releases](https://github.com/deslaughter/discon-wrapper/releases) and put it in the same directory as the Controller shared library. Start the server from the command line by running `discon-server_amd64.exe --port=8080 --debug=1`. The `port` argument specifies which port on your computer it will listen to for client connections. This can be any 4-5 digit number that is not already in use by the operating system. If it returns an error, try a different number. 

The `debug` argument now accepts a level (0, 1, or 2):
- `0`: No debug output (default)
- `1`: Basic debug information
- `2`: Verbose debug output including full payloads

That's it for the server configuration. It doesn't load the controller until the client makes a connection because the client has to tell it what to load, more on that in the next section. 

### Client

Download `discon-client_amd64.dll` from [Releases](https://github.com/deslaughter/discon-wrapper/releases) and put it in the same directory as the Controller shared library. This shared library takes the place of the controller and is loaded by OpenFAST. As such, the ServoDyn input file needs to be modified to point to this shared library. Change `DLL_FileName` to the path for `discon-client_amd64.dll`. `DLL_InFile` does not need to be changed. Change `DLL_ProcName` to `DISCON` as that is the name of the procedure in `discon-client_amd64.dll`.

Original:
```
"controller.dll"             DLL_FileName - Name/location of the dynamic library
"DISCON.IN"                  DLL_InFile   - Name of input file sent to the DLL (-)
"CONTROL"                    DLL_ProcName - Name of procedure in DLL to be called (-)
```

Modified
```
"discon-client_amd64.dll"    DLL_FileName - Name/location of the dynamic library
"DISCON.IN"                  DLL_InFile   - Name of input file sent to the DLL (-)
"DISCON"                     DLL_ProcName - Name of procedure in DLL to be called (-)
```

These changes will allow OpenFAST to load the DISCON-Wrapper client, but now we need to specify the Controller information so that the server knows what to load. The information is supplied with environment variables which will be used by the client when it is loaded by OpenFAST. The following command prompt input shows how to set the variables and run OpenFAST:

```
set DISCON_SERVER_ADDR=localhost:8080
set DISCON_LIB_PATH=controller.dll
set DISCON_LIB_PROC=CONTROL
set DISCON_CLIENT_DEBUG=1
openfast.exe my_turbine.fst
```

- `DISCON_SERVER_ADDR` describes the server address in one of these formats:
  - `hostname:port` (e.g. `localhost:8080`) - Standard WebSocket connection
  - `domain.name` (e.g. `controller.company.com`) - For use with reverse proxies that handle the port internally
  - `http://domain.name` - Explicit HTTP protocol, uses WebSocket (ws://)
  - `https://domain.name` - Secure HTTPS protocol, uses secure WebSocket (wss://)
- `DISCON_LIB_PATH` is the path from `discon-server_386.exe` to the controller shared library.
- `DISCON_LIB_PROC` is the procedure which will be called in the controller shared library.
- `DISCON_CLIENT_DEBUG` is used to enable debugging output on the client side, messages will be printed to the terminal.

You can see that the environment variable settings correspond to the original controller settings that were in the ServoDyn input file.

## Client-Side Input Files

DISCON-Wrapper now supports input files located on the client side. When a controller asks for an input file (such as DISCON.IN), the client will:

1. Check if the file exists locally
2. If found, automatically transfer the file to the server before the controller is called
3. Update the file path that's passed to the controller on the server side

This means you can now keep your input files on the client machine and don't need to manually copy them to the server. The file transfer happens automatically and transparently.

Benefits:
- No need to copy input files to the server
- Files are transferred only once during a simulation
- Files are automatically cleaned up when the connection closes

## Running

For simplicity, put the OpenFAST input files, controller shared library, `discon-server_386.exe`, and `discon-client_amd64.dll` into a directory. Open two command prompts, one for running the server, and the other for running OpenFAST. Start the server by running the following in one command prompt

```
discon-server_amd64.exe --port=8080 --debug=1
```

Switch to the second command prompt, specify the environment variables, and run OpenFAST:

```
set DISCON_SERVER_ADDR=localhost:8080
set DISCON_LIB_PATH=controller.dll
set DISCON_LIB_PROC=CONTROL
set DISCON_CLIENT_DEBUG=1
openfast.exe my_turbine.fst
```

If everything is configured properly, the simulation should proceed as though OpenFAST had loaded the controller directly. Remember, that the server must be started before running the simulation because the client will attempt to connect when it is loaded. Once the simulation stops, the client will disconnect. The server is will continue running and wait for new connections so you can run more simulations. To stop the server, switch to the command prompt, and close it or press `CTRL+C` to kill the server.

You may see temporary files created while the simulation is running. These include a copy of the controller and any transferred input files. They are automatically cleaned up when the connection closes, but if they aren't removed for some reason, they can be deleted manually once the server has been stopped.
