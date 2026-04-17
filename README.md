# openfakegps

This project is a tool coded interely by claude and is just a PoC but can be used into automations like apps that needs gps location tracking like Uber, delivery apps and etc.

I made this to can be extensible to other plataforms If needs.

Currently i am using only Android devices and navigator API to mock locations and keep It running into background and you can see It at folder called Android with kotlin app


This project uses gprc to communicate between multiple devices and the core is have a server that control devices using goroutines to be really quick and support multiple devices simulations. 

This tool is Very useful if you need simulate races scenarios like a Uber app.

The main concept here is be extensible to other plataforms If you want and all clients connected and controlled by the server but keep in mind that any custom implementation should follow the gprc especification até proto schema

# How use

you can use go compiler or just do docker compose up -d to build and run the core 

To build android app you should use android studio ith the newer sdk disponible

after build and install the app you need set the permission to mock location manually at developer options of you device

1. At docs folder you have a file called agent-tool-guide.md where you agent can use to reuse this project between yours automation
2. at api.md file you can found how this works
3. you always can use the /swagger endpoint to undestand api and get the open api especification and pass to you agent

feel free to contribute with IOS version

