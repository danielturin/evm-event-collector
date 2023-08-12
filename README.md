# evm-event-collector

**WARNING: WIP, DO NOT USE in production**

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/danielturin/evm-event-collector/blob/main/LICENSE)
![Go version](https://img.shields.io/badge/go-1.20-blue.svg)

## Overview
evm-event-collector utilized both native Golang, lock-free data structures (https://github.com/amirylm/lockfree) and the lock-free Reactor (https://github.com/amirylm/lockfree/reactor) to provide a performant mechanism for collecting, parsing and enqueuing Smart Contract event data by contract address. 

### Environment Configuration

.env file must contain the following properties:
* SOCKET_ADDRS: \<String Address providing websocket connectivity to the etherium blockchain infrastructure\>\n
  example: "wss://mainnet.infura.io/ws/v3/<INFURA_API_TOKEN>"
* TIMEOUT_DURATION: \<Desired connection timeout value in millisconds\>\n
  example: 10000000

### Collector Configuration
The following must be provided:

* config.json file located in root directory that must contain the following structure:
{
    "ABI": [
      {
        "name": <"Contract ABI Name">,
        "data": "<Escaped String of the complete contract ABI>" 
      }
    ],
    "events": [
      { 
          "addr": "<Contract Address>", 
          "eventSig": "<Hash value of the desired event type signature>", 
          "abi": "<Corresponding Contract ABI Name>" }
    ]
}

## Usage

```shell
go get github.com/danielturin/evm-event-collector
```

## Contributing

Contributions to evm-event-collector are welcomed and encouraged! If you find a bug or have an idea for an improvement, please open an issue or submit a pull request.

Before contributing, please make sure to read the [Contributing Guidelines](CONTRIBUTING.md) for more information.

## License

evm-event-collector is open-source software licensed under the [MIT License](LICENSE). Feel free to use, modify, and distribute this library as permitted by the license.
