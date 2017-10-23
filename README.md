# mqttstat
mqttstat is a command client to measure the cost of mqtt actions

```
$ ./mqttstat -server tcp://iot.eclipse.org:1883 ping
Connected to tcp://iot.eclipse.org:1883 from 172.17.0.2:36832

ClientID : mqttstat
CleanSession : true

  DNS Lookup   TCP Connection   MQTT Connection   MQTT PingPong
[26.239068ms |  273.02028ms   |   268.37164ms   | 274.074409ms  ]
             |                |                 |               |
        26.239068ms           |                 |               |
                        299.259348ms            |               |
                                          567.663153ms          |
                                                          841.737562ms
DNS Lookup                 26.239068ms  ███
TCP Connection             273.02028ms  ████████████████████████████████
MQTT Connection            268.37164ms  ███████████████████████████████
MQTT PingPong             274.074409ms  ████████████████████████████████

```
