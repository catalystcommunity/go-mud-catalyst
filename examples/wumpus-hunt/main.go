package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/catalystcommunity/muddycore/pkg/client"
	"github.com/catalystcommunity/muddycore/pkg/communication"
	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/message"
	"github.com/catalystcommunity/muddycore/pkg/server"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/auth"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/game"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/instance"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/portal"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/world"
	"github.com/urfave/cli/v2"
	"github.com/fxamacker/cbor/v2"
)

// Global server state
var (
	commManager *communication.CommunicationManager
	eventManager *events.DefaultEventManager
	instanceManager *instance.InstanceManager
	transportHandler *portal.TransportHandler
	gameStateManager *game.StateManager
	playerLocations = make(map[string]string) // clientID -> roomID
	playerLock sync.RWMutex
	// clientID -> playerID mapping for authentication
	clientToPlayer = make(map[string]string)
	clientLock sync.RWMutex
)

func runServer(ctx *cli.Context) error {
	// Configure server with both TCP and WebSocket support
	config := server.DefaultServerConfig()
	config.Host = ctx.String("host")
	config.Port = ctx.String("port")
	config.WebSocketPort = ctx.String("ws-port")
	config.EnableWebSocket = ctx.Bool("enable-ws")
	config.MaxConnections = ctx.Int("max-connections")

	// Initialize storage
	storageConfig := storage.DefaultStorageConfig()
	storageConfig.DataRoot = ctx.String("data-dir")
	storageMgr := storage.NewManagerWithConfig(storageConfig)
	if err := storageMgr.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize event manager
	eventManager = events.NewEventManager()

	// Initialize communication manager
	commManager = communication.NewCommunicationManager(storageMgr, eventManager)

	// Initialize instance manager
	instanceManager = instance.NewInstanceManager(storageMgr)
	
	// Initialize transport handler
	transportHandler = portal.NewTransportHandler(storageMgr, instanceManager)
	
	// Initialize game state manager
	gameStateManager = game.NewStateManager(storageMgr)

	// Create server
	s := server.NewServerWithConfig(config)

	// Set up event handlers for communication
	setupCommunicationHandlers(s, storageMgr)

	// Set up authentication and message handlers
	s.RegisterContextMessageHandler(message.MessageTypeAuth, func(clientID string, msg *message.Message) error {
		handleServerAuthentication(s, storageMgr, clientID, msg)
		return nil
	})
	
	s.RegisterContextMessageHandler(message.MessageTypeAction, func(clientID string, msg *message.Message) error {
		handleGameMessage(s, storageMgr, clientID, msg)
		return nil
	})
	
	logging.Info("Server created with authentication and game handlers registered")

	// Initialize world state (Inn, Portal rooms)
	if err := initializeWorld(storageMgr); err != nil {
		return fmt.Errorf("failed to initialize world: %w", err)
	}

	// Start server
	logging.Info("Starting Wumpus Hunt server", "host", config.Host, "port", config.Port)
	if err := s.StartServer(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logging.Info("Shutting down server...")
	s.Shutdown()
	
	// Shutdown managers
	if instanceManager != nil {
		instanceManager.Shutdown()
	}

	return nil
}

func runClient(ctx *cli.Context) error {
	// Configure client
	config := client.DefaultClientConfig(ctx.String("address"))
	config.ReconnectEnabled = ctx.Bool("reconnect")
	config.HeartbeatEnabled = ctx.Bool("heartbeat")

	// Create client
	c := client.NewClientWithConfig(config)

	// Set up event handlers
	c.SetConnectHandler(func() {
		fmt.Println("=== Welcome to Hunt the Wumpus! ===")
		fmt.Println("A dangerous adventure awaits in the depths...")
		fmt.Println()
		showMainMenu()
	})

	c.SetMessageHandler(func(msg *message.Message) {
		handleClientMessage(msg)
	})

	c.SetDisconnectHandler(func(err error) {
		if err != nil {
			fmt.Printf("Disconnected: %v\n", err)
		}
		fmt.Println("Connection lost. Goodbye!")
	})

	// Connect
	if err := c.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Start input handling
	go handleUserInput(c)

	// Wait for disconnect using a channel
	disconnectChan := make(chan error)
	c.SetDisconnectHandler(func(err error) {
		disconnectChan <- err
	})
	
	// Wait for disconnect
	<-disconnectChan

	return nil
}

func showMainMenu() {
	fmt.Println("=== Main Menu ===")
	fmt.Println("1. Login")
	fmt.Println("2. Create Account")
	fmt.Println("3. Exit")
	fmt.Print("Choose an option: ")
}

func handleUserInput(c *client.Client) {
	scanner := bufio.NewScanner(os.Stdin)
	authenticated := false

	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if !authenticated {
			switch input {
			case "1":
				handleLogin(c)
			case "2":
				handleCreateAccount(c)
			case "3":
				fmt.Println("Goodbye!")
				c.Disconnect()
				return
			default:
				fmt.Println("Invalid option. Please try again.")
				showMainMenu()
			}
		} else {
			// Handle authenticated user input
			handleGameInput(c, input)
		}
	}
}

func handleLogin(c *client.Client) {
	fmt.Print("Username: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	username := strings.TrimSpace(scanner.Text())

	fmt.Print("Password: ")
	scanner.Scan()
	password := strings.TrimSpace(scanner.Text())

	// Send authentication request
	authData := map[string]interface{}{
		"username": username,
		"password": password,
	}

	msg := message.NewMessage(message.MessageTypeAuth).WithContent(authData).Build()
	if err := c.SendMessage(msg); err != nil {
		fmt.Printf("Failed to send authentication: %v\n", err)
		showMainMenu()
		return
	}

	fmt.Println("Authenticating...")
}

func handleCreateAccount(c *client.Client) {
	fmt.Print("Choose a username: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	username := strings.TrimSpace(scanner.Text())

	if len(username) < 3 {
		fmt.Println("Username must be at least 3 characters long.")
		showMainMenu()
		return
	}

	fmt.Print("Choose a password (minimum 6 characters): ")
	scanner.Scan()
	password := strings.TrimSpace(scanner.Text())

	if len(password) < 6 {
		fmt.Println("Password must be at least 6 characters long.")
		showMainMenu()
		return
	}

	// Send authentication request (same as login, server will create account if needed)
	authData := map[string]interface{}{
		"username": username,
		"password": password,
	}

	msg := message.NewMessage(message.MessageTypeAuth).WithContent(authData).Build()
	if err := c.SendMessage(msg); err != nil {
		fmt.Printf("Failed to send authentication: %v\n", err)
		showMainMenu()
		return
	}

	fmt.Println("Creating account...")
}

func handleClientMessage(msg *message.Message) {
	switch msg.Type {
	case message.MessageTypeAuth:
		handleAuthResponse(msg)
	case message.MessageTypeChat:
		handleChatMessage(msg)
	case message.MessageTypeAction:
		handleGameResponse(msg)
	case message.MessageTypeError:
		handleSystemMessage(msg)
	// Handle communication system messages
	case communication.MessageTypeSay, communication.MessageTypeShout, communication.MessageTypeEmote, communication.MessageTypeTell:
		handleChatMessage(msg)
	default:
		// Handle other message types
		handleChatMessage(msg)
	}
}

func handleAuthResponse(msg *message.Message) {
	var response map[string]interface{}
	if err := cbor.Unmarshal(msg.Contents, &response); err != nil {
		fmt.Println("Authentication failed: Invalid response format")
		showMainMenu()
		return
	}

	success, ok := response["success"].(bool)
	if !ok || !success {
		errorMsg, _ := response["error"].(string)
		if errorMsg == "" {
			errorMsg = "Authentication failed"
		}
		fmt.Printf("Authentication failed: %s\n", errorMsg)
		showMainMenu()
		return
	}

	fmt.Println("Authentication successful!")
	fmt.Println()
	fmt.Println("You find yourself in The Inn, a warm and welcoming place...")
	fmt.Println("The air is filled with the scent of hearty stew and cheerful conversation.")
	fmt.Println("You can chat with other adventurers using 'say <message>' or 'shout <message>'.")
	fmt.Println("Type 'help' for available commands.")
	fmt.Print("> ")
}

func handleChatMessage(msg *message.Message) {
	// Handle communication system messages
	switch msg.Type {
	case communication.MessageTypeSay:
		var commData communication.CommunicationData
		if err := cbor.Unmarshal(msg.Contents, &commData); err == nil {
			if commData.SenderName != "" {
				fmt.Printf("%s says: %s\n", commData.SenderName, commData.Message)
			} else {
				fmt.Printf("Someone says: %s\n", commData.Message)
			}
		}
	case communication.MessageTypeShout:
		var commData communication.CommunicationData
		if err := cbor.Unmarshal(msg.Contents, &commData); err == nil {
			if commData.SenderName != "" {
				fmt.Printf("%s shouts: %s\n", commData.SenderName, commData.Message)
			} else {
				fmt.Printf("Someone shouts: %s\n", commData.Message)
			}
		}
	case communication.MessageTypeEmote:
		var commData communication.CommunicationData
		if err := cbor.Unmarshal(msg.Contents, &commData); err == nil {
			if commData.SenderName != "" {
				fmt.Printf("%s %s\n", commData.SenderName, commData.Message)
			} else {
				fmt.Printf("Someone %s\n", commData.Message)
			}
		}
	case communication.MessageTypeTell:
		var commData communication.CommunicationData
		if err := cbor.Unmarshal(msg.Contents, &commData); err == nil {
			if commData.SenderName != "" {
				fmt.Printf("%s tells you: %s\n", commData.SenderName, commData.Message)
			} else {
				fmt.Printf("Someone tells you: %s\n", commData.Message)
			}
		}
	default:
		// Handle legacy chat messages
		var chatData map[string]interface{}
		if err := cbor.Unmarshal(msg.Contents, &chatData); err == nil {
			if text, ok := chatData["text"].(string); ok {
				if sender, ok := chatData["sender"].(string); ok {
					fmt.Printf("[%s]: %s\n", sender, text)
				} else {
					fmt.Printf("Anonymous: %s\n", text)
				}
			}
		}
	}
	fmt.Print("> ")
}

func handleGameResponse(msg *message.Message) {
	var gameData map[string]interface{}
	if err := cbor.Unmarshal(msg.Contents, &gameData); err == nil {
		if text, ok := gameData["text"].(string); ok {
			fmt.Println(text)
		}
	}
	fmt.Print("> ")
}

func handleSystemMessage(msg *message.Message) {
	var systemData map[string]interface{}
	if err := cbor.Unmarshal(msg.Contents, &systemData); err == nil {
		if text, ok := systemData["text"].(string); ok {
			fmt.Printf("System: %s\n", text)
		}
	}
	fmt.Print("> ")
}

func handleGameInput(c *client.Client, input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	command := strings.ToLower(parts[0])
	args := parts[1:]

	switch command {
	case "help":
		showGameHelp()
	case "say", "chat":
		if len(args) > 0 {
			text := strings.Join(args, " ")
			// Use the communication system for room chat
			commData := communication.CommunicationData{
				Message: text,
				SenderName: "Player", // Will be updated by server
			}
			contents, _ := cbor.Marshal(commData)
			chatMsg := message.NewMessage(communication.MessageTypeSay).WithContent(contents).Build()
			c.SendMessage(chatMsg)
		} else {
			fmt.Println("Usage: say <message>")
		}
	case "shout":
		if len(args) > 0 {
			text := strings.Join(args, " ")
			// Use the communication system for world chat
			commData := communication.CommunicationData{
				Message: text,
				SenderName: "Player", // Will be updated by server
			}
			contents, _ := cbor.Marshal(commData)
			chatMsg := message.NewMessage(communication.MessageTypeShout).WithContent(contents).Build()
			c.SendMessage(chatMsg)
		} else {
			fmt.Println("Usage: shout <message>")
		}
	case "emote":
		if len(args) > 0 {
			text := strings.Join(args, " ")
			// Use the communication system for emotes
			commData := communication.CommunicationData{
				Message: text,
				SenderName: "Player", // Will be updated by server
			}
			contents, _ := cbor.Marshal(commData)
			chatMsg := message.NewMessage(communication.MessageTypeEmote).WithContent(contents).Build()
			c.SendMessage(chatMsg)
		} else {
			fmt.Println("Usage: emote <action> (e.g., emote waves)")
		}
	case "tell":
		if len(args) > 1 {
			target := args[0]
			text := strings.Join(args[1:], " ")
			// Use the communication system for private messages
			commData := communication.CommunicationData{
				Message: text,
				SenderName: "Player", // Will be updated by server
				TargetName: target,
			}
			contents, _ := cbor.Marshal(commData)
			chatMsg := message.NewMessage(communication.MessageTypeTell).WithContent(contents).Build()
			c.SendMessage(chatMsg)
		} else {
			fmt.Println("Usage: tell <player> <message>")
		}
	case "look":
		gameMsg := message.NewMessage(message.MessageTypeAction).WithContent(map[string]interface{}{
			"action": "look",
		}).Build()
		c.SendMessage(gameMsg)
	case "stats":
		gameMsg := message.NewMessage(message.MessageTypeAction).WithContent(map[string]interface{}{
			"action": "stats",
		}).Build()
		c.SendMessage(gameMsg)
	case "inventory", "inv":
		gameMsg := message.NewMessage(message.MessageTypeAction).WithContent(map[string]interface{}{
			"action": "inventory",
		}).Build()
		c.SendMessage(gameMsg)
	case "go", "move":
		if len(args) > 0 {
			direction := strings.ToLower(args[0])
			gameMsg := message.NewMessage(message.MessageTypeAction).WithContent(map[string]interface{}{
				"action": "move",
				"direction": direction,
			}).Build()
			c.SendMessage(gameMsg)
		} else {
			fmt.Println("Usage: go <direction> (e.g., go north)")
		}
	case "north", "south", "east", "west", "northeast", "northwest", "southeast", "southwest", "up", "down":
		// Allow direct directional movement
		gameMsg := message.NewMessage(message.MessageTypeAction).WithContent(map[string]interface{}{
			"action": "move",
			"direction": command,
		}).Build()
		c.SendMessage(gameMsg)
	case "portal":
		if len(args) > 0 {
			portalCommand := strings.ToLower(args[0])
			gameMsg := message.NewMessage(message.MessageTypeAction).WithContent(map[string]interface{}{
				"action": "portal",
				"command": portalCommand,
			}).Build()
			c.SendMessage(gameMsg)
		} else {
			fmt.Println("Usage: portal <command> (enter/exit/status)")
			fmt.Println("  portal enter - Enter the portal to hunt the wumpus")
			fmt.Println("  portal exit  - Exit back to the Inn")
			fmt.Println("  portal status - Check your current portal status")
		}
	case "enter":
		// Quick portal enter command
		gameMsg := message.NewMessage(message.MessageTypeAction).WithContent(map[string]interface{}{
			"action": "portal",
			"command": "enter",
		}).Build()
		c.SendMessage(gameMsg)
	case "heal":
		// Heal command (only works in Inn)
		gameMsg := message.NewMessage(message.MessageTypeAction).WithContent(map[string]interface{}{
			"action": "heal",
		}).Build()
		c.SendMessage(gameMsg)
	case "attack":
		if len(args) > 0 {
			target := strings.ToLower(args[0])
			if target == "wumpus" {
				gameMsg := message.NewMessage(message.MessageTypeAction).WithContent(map[string]interface{}{
					"action": "attack",
					"target": "wumpus",
				}).Build()
				c.SendMessage(gameMsg)
			} else {
				fmt.Printf("You can't attack '%s'. Currently only 'attack wumpus' is supported.\n", target)
			}
		} else {
			fmt.Println("Usage: attack <target> (e.g., attack wumpus)")
		}
	case "combat":
		// Show combat status
		gameMsg := message.NewMessage(message.MessageTypeAction).WithContent(map[string]interface{}{
			"action": "combat_status",
		}).Build()
		c.SendMessage(gameMsg)
	case "quit", "exit":
		fmt.Println("Goodbye!")
		c.Disconnect()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Type 'help' for available commands.")
	}
}

func showGameHelp() {
	fmt.Println("=== Available Commands ===")
	fmt.Println("help        - Show this help message")
	fmt.Println("say <msg>   - Send a message to players in the same room")
	fmt.Println("shout <msg> - Send a message to all players in the world")
	fmt.Println("emote <act> - Perform an action (e.g., emote waves)")
	fmt.Println("tell <who> <msg> - Send a private message to a player")
	fmt.Println("look        - Look around your current location")
	fmt.Println("stats       - Show your character statistics")
	fmt.Println("inventory   - Show your inventory")
	fmt.Println("go <dir>    - Move in a direction (e.g., go north)")
	fmt.Println("north/south - Move north or south (shorthand)")
	fmt.Println("portal <cmd> - Portal commands (enter/exit/status)")
	fmt.Println("enter       - Quick command to enter the portal")
	fmt.Println("heal        - Restore health (only works in The Inn)")
	fmt.Println("attack <target> - Attack a target (e.g., attack wumpus)")
	fmt.Println("combat      - Show current combat status")
	fmt.Println("quit/exit   - Leave the game")
	fmt.Println()
}

func handleGameMessage(s *server.ConnServer, storageMgr *storage.Manager, clientID string, msg *message.Message) {
	// Handle communication messages first
	if msg.Type == communication.MessageTypeSay || msg.Type == communication.MessageTypeShout || 
	   msg.Type == communication.MessageTypeEmote || msg.Type == communication.MessageTypeTell {
		// Get player ID for this client
		playerID := getPlayerByClientID(clientID)
		if playerID == "" {
			// Not authenticated, ignore communication
			return
		}

		// Update communication data with proper sender info
		var commData communication.CommunicationData
		if err := cbor.Unmarshal(msg.Contents, &commData); err != nil {
			return
		}

		// Load player to get name
		player, err := storageMgr.Players().Load(playerID)
		if err != nil {
			return
		}

		// Update sender information
		commData.SenderID = playerID
		commData.SenderName = player.DisplayName

		// Update message contents
		updatedContents, err := cbor.Marshal(commData)
		if err != nil {
			return
		}
		msg.Contents = updatedContents

		// Process through communication manager
		if err := commManager.HandleMessage(msg, playerID); err != nil {
			sendGameMessage(s, clientID, fmt.Sprintf("Communication error: %v", err))
		}
		return
	}

	// Handle other game actions
	var gameData map[string]interface{}
	if err := cbor.Unmarshal(msg.Contents, &gameData); err != nil {
		return
	}

	action, ok := gameData["action"].(string)
	if !ok {
		return
	}

	switch action {
	case "look":
		handleLookAction(s, storageMgr, clientID)
	case "stats":
		handleStatsAction(s, storageMgr, clientID)
	case "inventory":
		handleInventoryAction(s, storageMgr, clientID)
	case "move":
		if direction, ok := gameData["direction"].(string); ok {
			handleMoveAction(s, storageMgr, clientID, direction)
		} else {
			sendGameMessage(s, clientID, "Invalid move command.")
		}
	case "portal":
		if command, ok := gameData["command"].(string); ok {
			handlePortalAction(s, storageMgr, clientID, command)
		} else {
			sendGameMessage(s, clientID, "Invalid portal command.")
		}
	case "heal":
		handleHealAction(s, storageMgr, clientID)
	case "attack":
		if target, ok := gameData["target"].(string); ok {
			handleAttackAction(s, storageMgr, clientID, target)
		} else {
			sendGameMessage(s, clientID, "Invalid attack command.")
		}
	case "combat_status":
		handleCombatStatusAction(s, storageMgr, clientID)
	}
}

func handleLookAction(s *server.ConnServer, storageMgr *storage.Manager, clientID string) {
	// Get player's current location
	currentRoomID, exists := getPlayerLocation(clientID)
	if !exists {
		sendGameMessage(s, clientID, "You don't seem to be anywhere. This is a bug - please report it.")
		return
	}
	
	// Get the main world (for now, assume all rooms are in main world)
	mainWorld, err := world.GetMainWorld(storageMgr)
	if err != nil {
		sendGameMessage(s, clientID, "Error: Could not find the game world.")
		return
	}
	
	// Get current room
	currentRoom, err := storageMgr.Worlds().LoadRoom(mainWorld.ID, currentRoomID)
	if err != nil {
		sendGameMessage(s, clientID, "Error: Could not find your current location.")
		return
	}
	
	// Send room description
	sendGameMessage(s, clientID, fmt.Sprintf("%s\n%s", currentRoom.Name, currentRoom.Description))
	
	// Show available exits
	if len(currentRoom.Exits) > 0 {
		exits := make([]string, 0, len(currentRoom.Exits))
		for _, exit := range currentRoom.Exits {
			exits = append(exits, exit.Direction)
		}
		sendGameMessage(s, clientID, fmt.Sprintf("\nExits: %s", strings.Join(exits, ", ")))
	} else {
		sendGameMessage(s, clientID, "\nThere are no obvious exits.")
	}
}

func handleStatsAction(s *server.ConnServer, storageMgr *storage.Manager, clientID string) {
	// Get player by client ID
	playerID := getPlayerByClientID(clientID)
	if playerID == "" {
		sendGameMessage(s, clientID, "Error: Player not found. Please authenticate.")
		return
	}

	// Load player
	player, err := storageMgr.Players().Load(playerID)
	if err != nil {
		sendGameMessage(s, clientID, "Error loading player data.")
		return
	}

	// Get current stats and state
	stats := auth.GetWumpusStats(player)
	health := gameStateManager.GetPlayerHealth(player)
	isInMaze := gameStateManager.IsPlayerInMaze(player)
	
	statsText := fmt.Sprintf("=== Character Statistics ===\n"+
		"Health: %d/100\n"+
		"Games Played: %d\n"+
		"Games Won: %d\n"+
		"Wumpus Pelts: %d\n"+
		"Total Deaths: %d\n"+
		"Currently in Maze: %v",
		health, stats.GamesPlayed, stats.GamesWon, stats.WumpusPelts, stats.TotalDeaths, isInMaze)
	
	sendGameMessage(s, clientID, statsText)
}

func handleHealAction(s *server.ConnServer, storageMgr *storage.Manager, clientID string) {
	// Get player by client ID
	playerID := getPlayerByClientID(clientID)
	if playerID == "" {
		sendGameMessage(s, clientID, "Error: Player not found. Please authenticate.")
		return
	}

	// Load player
	player, err := storageMgr.Players().Load(playerID)
	if err != nil {
		sendGameMessage(s, clientID, "Error loading player data.")
		return
	}

	// Check if player is in The Inn (healing only works there)
	currentRoomID, exists := getPlayerLocation(clientID)
	if !exists {
		sendGameMessage(s, clientID, "You don't seem to be anywhere. This is a bug - please report it.")
		return
	}

	// Get the main world and Inn room
	mainWorld, err := world.GetMainWorld(storageMgr)
	if err != nil {
		sendGameMessage(s, clientID, "Error: Could not find the game world.")
		return
	}

	innRoom, err := world.GetInnRoom(storageMgr, mainWorld.ID)
	if err != nil {
		sendGameMessage(s, clientID, "Error: Could not find The Inn.")
		return
	}

	// Check if player is in The Inn
	if currentRoomID != innRoom.ID {
		sendGameMessage(s, clientID, "You can only heal in The Inn. Return there to restore your health.")
		return
	}

	// Heal the player
	result, err := gameStateManager.FullyHealPlayer(player)
	if err != nil {
		sendGameMessage(s, clientID, fmt.Sprintf("Healing failed: %v", err))
		return
	}

	sendGameMessage(s, clientID, result.Message)
}

func handleAttackAction(s *server.ConnServer, storageMgr *storage.Manager, clientID, target string) {
	// Get player by client ID
	playerID := getPlayerByClientID(clientID)
	if playerID == "" {
		sendGameMessage(s, clientID, "Error: Player not found. Please authenticate.")
		return
	}

	// Load player
	player, err := storageMgr.Players().Load(playerID)
	if err != nil {
		sendGameMessage(s, clientID, "Error loading player data.")
		return
	}

	// Check if player is alive
	if !gameStateManager.IsPlayerAlive(player) {
		sendGameMessage(s, clientID, "You cannot attack while dead. Return to The Inn to heal.")
		return
	}

	// Only handle wumpus attacks for now
	if target != "wumpus" {
		sendGameMessage(s, clientID, fmt.Sprintf("You cannot attack '%s'. Currently only 'attack wumpus' is supported.", target))
		return
	}

	// Get player's current location
	currentRoomID, exists := getPlayerLocation(clientID)
	if !exists {
		sendGameMessage(s, clientID, "You don't seem to be anywhere. This is a bug - please report it.")
		return
	}
	
	// Get the current room - we need to determine which world it's in
	// Try main world first, then check if player is in a maze instance
	var currentRoom *storage.Room
	var roomErr error
	
	if gameStateManager.IsPlayerInMaze(player) {
		// Player is in a maze, need to find the right world
		instanceID := gameStateManager.GetPlayerCurrentInstance(player)
		if instanceID != "" {
			// Try to get the instance info to find the world
			instanceInfo, err := instanceManager.GetInstanceInfo(instanceID)
			if err == nil {
				currentRoom, roomErr = storageMgr.Worlds().LoadRoom(instanceInfo.WorldID, currentRoomID)
			}
		}
	}
	
	if currentRoom == nil {
		// Try main world as fallback
		mainWorld, err := world.GetMainWorld(storageMgr)
		if err != nil {
			sendGameMessage(s, clientID, "Error: Could not find the game world.")
			return
		}
		currentRoom, roomErr = storageMgr.Worlds().LoadRoom(mainWorld.ID, currentRoomID)
	}
	
	if roomErr != nil {
		sendGameMessage(s, clientID, "Error: Could not find your current location.")
		return
	}

	// Attempt to attack the wumpus
	result, err := gameStateManager.AttackWumpus(player, currentRoom)
	if err != nil {
		sendGameMessage(s, clientID, fmt.Sprintf("Attack failed: %v", err))
		return
	}

	// Send result message
	sendGameMessage(s, clientID, result.Message)

	// Handle death or victory
	if result.PlayerDied || result.PlayerWon {
		setPlayerLocation(clientID, result.NewLocation)
		
		// Update communication manager with new location
		if result.NewWorldID != "" {
			commManager.UpdatePlayerLocation(playerID, result.NewWorldID, result.NewLocation)
		}
		
		// Reset combat state on death or victory
		gameStateManager.ResetCombatState(player)
		
		// If player won, update their maze state
		if result.PlayerWon {
			if err := gameStateManager.SetPlayerInMaze(player, false, ""); err != nil {
				logging.Error("Failed to set player out of maze after victory", "error", err)
			}
		}
	}
}

func handleCombatStatusAction(s *server.ConnServer, storageMgr *storage.Manager, clientID string) {
	// Get player by client ID
	playerID := getPlayerByClientID(clientID)
	if playerID == "" {
		sendGameMessage(s, clientID, "Error: Player not found. Please authenticate.")
		return
	}

	// Load player
	player, err := storageMgr.Players().Load(playerID)
	if err != nil {
		sendGameMessage(s, clientID, "Error loading player data.")
		return
	}

	// Get combat state
	combatState := gameStateManager.GetCombatState(player)
	health := gameStateManager.GetPlayerHealth(player)
	isAlive := gameStateManager.IsPlayerAlive(player)
	
	statusText := fmt.Sprintf("=== Combat Status ===\n"+
		"Health: %d/100 (%s)\n"+
		"Total Damage Dealt: %d\n"+
		"Attacks Made: %d\n"+
		"Status: %s",
		health, gameStateManager.GetPlayerHealthStatus(player),
		combatState.TotalDamageDealt, combatState.AttackCount,
		func() string {
			if !isAlive {
				return "Dead"
			} else if combatState.TotalDamageDealt > 0 {
				return "In Combat"
			} else {
				return "Ready for Combat"
			}
		}())
	
	sendGameMessage(s, clientID, statusText)
}

func handleInventoryAction(s *server.ConnServer, storageMgr *storage.Manager, clientID string) {
	// Get player by client ID (placeholder implementation)
	// TODO: Implement actual player lookup by clientID
	// player := getPlayerByClientID(clientID)
	// inventoryText := items.FormatInventoryList(player)
	
	// For now, show empty inventory
	inventoryText := "Your inventory is empty."
	
	response := message.NewMessage(message.MessageTypeAction).WithContent(map[string]interface{}{
		"text": inventoryText,
	}).Build()
	s.SendMessage(clientID, response)
}

func initializeWorld(storageMgr *storage.Manager) error {
	// Try to get existing main world first
	if mainWorld, err := world.GetMainWorld(storageMgr); err == nil {
		// World exists, validate its structure
		if err := world.ValidateWorldStructure(mainWorld, storageMgr); err != nil {
			logging.Warn("World structure validation failed, recreating", "error", err)
		} else {
			logging.Info("World already exists and is valid")
			return nil
		}
	}

	// Create new main world with Inn and Portal
	mainWorld, err := world.CreateMainWorld(storageMgr)
	if err != nil {
		return fmt.Errorf("failed to create main world: %w", err)
	}

	logging.Info("World initialized successfully", "world_id", mainWorld.ID)
	return nil
}

func handleServerAuthentication(s *server.ConnServer, storageMgr *storage.Manager, clientID string, msg *message.Message) {
	var authData map[string]interface{}
	if err := cbor.Unmarshal(msg.Contents, &authData); err != nil {
		sendAuthResponse(s, clientID, false, "Invalid authentication data format")
		return
	}

	username, ok := authData["username"].(string)
	if !ok {
		sendAuthResponse(s, clientID, false, "Username is required")
		return
	}

	password, ok := authData["password"].(string)
	if !ok {
		sendAuthResponse(s, clientID, false, "Password is required")
		return
	}

	// Try to load existing player
	player, err := storageMgr.Players().LoadByUsername(username)
	if err != nil {
		// Player doesn't exist, create new one
		player = storage.NewPlayer(ids.NewEntityID(), username)
		player.DisplayName = username
		
		// Set password
		if err := auth.SetPlayerPassword(player, password); err != nil {
			sendAuthResponse(s, clientID, false, "Failed to set password")
			return
		}
		
		// Save new player
		if err := storageMgr.Players().Save(player); err != nil {
			sendAuthResponse(s, clientID, false, "Failed to create account")
			return
		}
		
		logging.Info("Created new player account", "username", username, "playerID", player.ID)
	} else {
		// Player exists, validate password
		if !auth.ValidatePlayerPassword(player, password) {
			sendAuthResponse(s, clientID, false, "Invalid username or password")
			return
		}
		
		logging.Info("Player authenticated", "username", username, "playerID", player.ID)
	}

	// Set client-to-player mapping
	setClientToPlayer(clientID, player.ID)

	// Initialize player in the Inn
	if err := initializePlayerInInn(storageMgr, clientID, player.ID); err != nil {
		logging.Error("Failed to initialize player in Inn", "error", err)
		sendAuthResponse(s, clientID, false, "Failed to initialize player location")
		return
	}

	// Send success response
	sendAuthResponse(s, clientID, true, "Authentication successful")
}

func sendAuthResponse(s *server.ConnServer, clientID string, success bool, responseMessage string) {
	response := map[string]interface{}{
		"success": success,
	}
	if responseMessage != "" {
		if success {
			response["message"] = responseMessage
		} else {
			response["error"] = responseMessage
		}
	}
	
	msg := message.NewMessage(message.MessageTypeAuth).WithContent(response).Build()
	s.SendMessage(clientID, msg)
}

func handlePortalAction(s *server.ConnServer, storageMgr *storage.Manager, clientID, command string) {
	// Get player by client ID
	playerID := getPlayerByClientID(clientID)
	if playerID == "" {
		sendGameMessage(s, clientID, "Error: Player not found. Please authenticate.")
		return
	}

	// Load player
	player, err := storageMgr.Players().Load(playerID)
	if err != nil {
		sendGameMessage(s, clientID, "Error loading player data.")
		return
	}

	// Handle portal command
	result, err := transportHandler.HandlePortalCommand(player, command)
	if err != nil {
		sendGameMessage(s, clientID, fmt.Sprintf("Portal error: %v", err))
		return
	}

	// Send result message
	sendGameMessage(s, clientID, result.Message)

	// If transport was successful, update player location and maze state
	if result.Success && result.TargetRoomID != "" {
		setPlayerLocation(clientID, result.TargetRoomID)
		
		// Update communication manager with new location
		if result.TargetWorldID != "" {
			commManager.UpdatePlayerLocation(playerID, result.TargetWorldID, result.TargetRoomID)
		}
		
		// Update player's maze state based on portal command
		if command == "enter" && result.InstanceID != "" {
			// Player entered a maze instance
			if err := gameStateManager.SetPlayerInMaze(player, true, result.InstanceID); err != nil {
				logging.Error("Failed to set player in maze", "error", err)
			}
		} else if command == "exit" {
			// Player exited maze
			if err := gameStateManager.SetPlayerInMaze(player, false, ""); err != nil {
				logging.Error("Failed to set player out of maze", "error", err)
			}
		}
		
		// If player entered a new instance, show the room description
		if result.TargetWorldID != "" && result.TargetRoomID != "" {
			// Load the target room and show its description
			targetRoom, err := storageMgr.Worlds().LoadRoom(result.TargetWorldID, result.TargetRoomID)
			if err == nil {
				sendGameMessage(s, clientID, fmt.Sprintf("\n%s\n%s", targetRoom.Name, targetRoom.Description))
				
				// Get sensory messages for atmospheric effects
				sensoryMessages := gameStateManager.GetSensoryMessages(targetRoom, storageMgr)
				for _, msg := range sensoryMessages {
					sendGameMessage(s, clientID, msg)
				}
				
				// Show available exits
				if len(targetRoom.Exits) > 0 {
					exits := make([]string, 0, len(targetRoom.Exits))
					for _, exit := range targetRoom.Exits {
						exits = append(exits, exit.Direction)
					}
					sendGameMessage(s, clientID, fmt.Sprintf("\nExits: %s", strings.Join(exits, ", ")))
				} else {
					sendGameMessage(s, clientID, "\nThere are no obvious exits.")
				}
			}
		}
	}
}

// Helper functions for player location management
func setPlayerLocation(clientID, roomID string) {
	playerLock.Lock()
	defer playerLock.Unlock()
	playerLocations[clientID] = roomID
}

func getPlayerLocation(clientID string) (string, bool) {
	playerLock.RLock()
	defer playerLock.RUnlock()
	roomID, exists := playerLocations[clientID]
	return roomID, exists
}

func removePlayerLocation(clientID string) {
	playerLock.Lock()
	defer playerLock.Unlock()
	delete(playerLocations, clientID)
}

// Helper function to get Inn room from storage
func getInnRoomFromStorage(storageMgr *storage.Manager) (*storage.Room, error) {
	// Get the main world first
	mainWorld, err := world.GetMainWorld(storageMgr)
	if err != nil {
		return nil, fmt.Errorf("failed to get main world: %w", err)
	}
	
	// Get the Inn room from the main world
	innRoom, err := world.GetInnRoom(storageMgr, mainWorld.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Inn room: %w", err)
	}
	
	return innRoom, nil
}

// Initialize player in the Inn when they first connect
func initializePlayerInInn(storageMgr *storage.Manager, clientID, playerID string) error {
	// Get the Inn room
	innRoom, err := getInnRoomFromStorage(storageMgr)
	if err != nil {
		return fmt.Errorf("failed to get Inn room: %w", err)
	}
	
	// Get the main world
	mainWorld, err := world.GetMainWorld(storageMgr)
	if err != nil {
		return fmt.Errorf("failed to get main world: %w", err)
	}
	
	// Set player location in both systems
	setPlayerLocation(clientID, innRoom.ID)
	setClientToPlayer(clientID, playerID)
	commManager.UpdatePlayerLocation(playerID, mainWorld.ID, innRoom.ID)
	
	return nil
}

// Helper function to send game messages
func sendGameMessage(s *server.ConnServer, clientID, text string) {
	response := message.NewMessage(message.MessageTypeAction).WithContent(map[string]interface{}{
		"text": text,
	}).Build()
	s.SendMessage(clientID, response)
}

// Handle player movement between rooms
func handleMoveAction(s *server.ConnServer, storageMgr *storage.Manager, clientID, direction string) {
	// Get player's current location
	currentRoomID, exists := getPlayerLocation(clientID)
	if !exists {
		sendGameMessage(s, clientID, "You don't seem to be anywhere. This is a bug - please report it.")
		return
	}
	
	// Get the main world (for now, assume all rooms are in main world)
	mainWorld, err := world.GetMainWorld(storageMgr)
	if err != nil {
		sendGameMessage(s, clientID, "Error: Could not find the game world.")
		return
	}
	
	// Get current room
	currentRoom, err := storageMgr.Worlds().LoadRoom(mainWorld.ID, currentRoomID)
	if err != nil {
		sendGameMessage(s, clientID, "Error: Could not find your current location.")
		return
	}
	
	// Check if movement direction is valid
	exit := currentRoom.GetExit(direction)
	if exit == nil {
		sendGameMessage(s, clientID, fmt.Sprintf("You can't go %s from here.", direction))
		return
	}
	
	// Get target room
	targetRoom, err := storageMgr.Worlds().LoadRoom(exit.TargetWorldID, exit.TargetRoomID)
	if err != nil {
		sendGameMessage(s, clientID, "Error: Could not find the destination.")
		return
	}
	
	// Get player for game state processing
	playerID := getPlayerByClientID(clientID)
	if playerID == "" {
		sendGameMessage(s, clientID, "Error: Player not found. Please authenticate.")
		return
	}

	player, err := storageMgr.Players().Load(playerID)
	if err != nil {
		sendGameMessage(s, clientID, "Error loading player data.")
		return
	}

	// Check if player is alive before allowing movement
	if !gameStateManager.IsPlayerAlive(player) {
		sendGameMessage(s, clientID, "You are dead and cannot move. You should be automatically respawned.")
		return
	}

	// Move player to new room
	setPlayerLocation(clientID, exit.TargetRoomID)
	commManager.UpdatePlayerLocation(playerID, exit.TargetWorldID, exit.TargetRoomID)
	
	// Send movement message and room description
	sendGameMessage(s, clientID, fmt.Sprintf("You head %s.", direction))
	sendGameMessage(s, clientID, fmt.Sprintf("\n%s\n%s", targetRoom.Name, targetRoom.Description))
	
	// Process room hazards
	hazardResult, err := gameStateManager.ProcessRoomHazards(player, targetRoom)
	if err != nil {
		logging.Error("Error processing room hazards", "error", err)
	} else if hazardResult.Message != "" {
		sendGameMessage(s, clientID, hazardResult.Message)
		
		// If player died from hazard, handle respawn
		if hazardResult.PlayerDied {
			setPlayerLocation(clientID, hazardResult.NewLocation)
			commManager.UpdatePlayerLocation(playerID, hazardResult.NewWorldID, hazardResult.NewLocation)
			return // Don't show exits or sensory messages
		}
	}
	
	// Check for wumpus encounter (only if player is still alive)
	if gameStateManager.IsPlayerAlive(player) {
		wumpusResult, err := gameStateManager.CheckWumpusEncounter(player, targetRoom)
		if err != nil {
			logging.Error("Error processing wumpus encounter", "error", err)
		} else if wumpusResult.Message != "" {
			sendGameMessage(s, clientID, wumpusResult.Message)
			
			// Handle death or victory
			if wumpusResult.PlayerDied || wumpusResult.PlayerWon {
				setPlayerLocation(clientID, wumpusResult.NewLocation)
				commManager.UpdatePlayerLocation(playerID, wumpusResult.NewWorldID, wumpusResult.NewLocation)
				return // Don't show exits or sensory messages
			}
		}
	}
	
	// Get sensory messages for atmospheric effects
	sensoryMessages := gameStateManager.GetSensoryMessages(targetRoom, storageMgr)
	for _, msg := range sensoryMessages {
		sendGameMessage(s, clientID, msg)
	}
	
	// Show available exits
	if len(targetRoom.Exits) > 0 {
		exits := make([]string, 0, len(targetRoom.Exits))
		for _, exit := range targetRoom.Exits {
			exits = append(exits, exit.Direction)
		}
		sendGameMessage(s, clientID, fmt.Sprintf("\nExits: %s", strings.Join(exits, ", ")))
	} else {
		sendGameMessage(s, clientID, "\nThere are no obvious exits.")
	}
}

// setupCommunicationHandlers sets up event handlers for the communication system
func setupCommunicationHandlers(s *server.ConnServer, storageMgr *storage.Manager) {
	// Handle communication events
	eventManager.RegisterHook(events.EventTypeMessageSent, func(event events.Event) events.EventResult {
		// Extract message data from event
		if eventData := event.Data(); eventData != nil {
			if msg, ok := eventData["message"].(*message.Message); ok {
				if messageType, ok := eventData["message_type"].(string); ok {
					switch messageType {
					case "room":
						if recipients, ok := eventData["recipients"].([]string); ok {
							for _, playerID := range recipients {
								if clientID := getClientByPlayerID(playerID); clientID != "" {
									s.SendMessage(clientID, msg)
								}
							}
						}
					case "world":
						if recipients, ok := eventData["recipients"].([]string); ok {
							for _, playerID := range recipients {
								if clientID := getClientByPlayerID(playerID); clientID != "" {
									s.SendMessage(clientID, msg)
								}
							}
						}
					case "private":
						if recipient, ok := eventData["recipient"].(string); ok {
							if clientID := getClientByPlayerID(recipient); clientID != "" {
								s.SendMessage(clientID, msg)
							}
						}
					}
				}
			}
		}
		return events.EventResultContinue
	}, 0)

	// Handle whisper target resolution
	eventManager.RegisterHook(events.EventTypeCustomPrefix+"resolve_whisper_target", func(event events.Event) events.EventResult {
		if eventData := event.Data(); eventData != nil {
			if targetName, ok := eventData["target_name"].(string); ok {
				if msg, ok := eventData["message"].(*message.Message); ok {
					// Find player by name
					if targetPlayerID := findPlayerByName(storageMgr, targetName); targetPlayerID != "" {
						// Send message to target
						if clientID := getClientByPlayerID(targetPlayerID); clientID != "" {
							s.SendMessage(clientID, msg)
						}
					} else {
						// Send error back to sender
						if senderID, ok := eventData["sender_id"].(string); ok {
							if clientID := getClientByPlayerID(senderID); clientID != "" {
								errorMsg := message.NewMessage(message.MessageTypeError).WithContent(map[string]interface{}{
									"text": fmt.Sprintf("Player '%s' not found.", targetName),
								}).Build()
								s.SendMessage(clientID, errorMsg)
							}
						}
					}
				}
			}
		}
		return events.EventResultContinue
	}, 0)
}

// Helper function to get client ID by player ID
func getClientByPlayerID(playerID string) string {
	clientLock.RLock()
	defer clientLock.RUnlock()
	for clientID, pID := range clientToPlayer {
		if pID == playerID {
			return clientID
		}
	}
	return ""
}

// Helper function to find player by name
func findPlayerByName(storageMgr *storage.Manager, playerName string) string {
	// In a real implementation, this would query the storage for players by name
	// For now, we'll return empty string as a placeholder
	// TODO: Implement player lookup by name
	return ""
}

// Helper function to set client-to-player mapping
func setClientToPlayer(clientID, playerID string) {
	clientLock.Lock()
	defer clientLock.Unlock()
	clientToPlayer[clientID] = playerID
}

// Helper function to get player ID by client ID
func getPlayerByClientID(clientID string) string {
	clientLock.RLock()
	defer clientLock.RUnlock()
	return clientToPlayer[clientID]
}

// Helper function to remove client-to-player mapping
func removeClientToPlayer(clientID string) {
	clientLock.Lock()
	defer clientLock.Unlock()
	if playerID, exists := clientToPlayer[clientID]; exists {
		// Remove from communication manager
		commManager.RemovePlayerLocation(playerID)
		delete(clientToPlayer, clientID)
	}
	// Also remove from player locations
	removePlayerLocation(clientID)
}

func main() {
	app := &cli.App{
		Name:  "wumpus-hunt",
		Usage: "Hunt the Wumpus - A MuddyCore Example",
		Commands: []*cli.Command{
			{
				Name:    "server",
				Aliases: []string{"s", "serve"},
				Usage:   "Start the Wumpus Hunt server",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "host",
						Value: "localhost",
						Usage: "Server host address",
					},
					&cli.StringFlag{
						Name:  "port",
						Value: "7777",
						Usage: "TCP server port",
					},
					&cli.StringFlag{
						Name:  "ws-port",
						Value: "7778",
						Usage: "WebSocket server port",
					},
					&cli.BoolFlag{
						Name:  "enable-ws",
						Value: false,
						Usage: "Enable WebSocket server",
					},
					&cli.IntFlag{
						Name:  "max-connections",
						Value: 100,
						Usage: "Maximum concurrent connections",
					},
					&cli.StringFlag{
						Name:  "data-dir",
						Value: "./data",
						Usage: "Data storage directory",
					},
				},
				Action: runServer,
			},
			{
				Name:    "client",
				Aliases: []string{"c", "connect"},
				Usage:   "Connect to Wumpus Hunt server",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "address",
						Value: "localhost:7777",
						Usage: "Server address to connect to",
					},
					&cli.BoolFlag{
						Name:  "reconnect",
						Value: true,
						Usage: "Enable automatic reconnection",
					},
					&cli.BoolFlag{
						Name:  "heartbeat",
						Value: true,
						Usage: "Enable heartbeat messages",
					},
				},
				Action: runClient,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logging.Fatal("Application failed", "error", err)
	}
}