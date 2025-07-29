# Wumpus Hunt - MuddyCore Example

A comprehensive multiplayer MUD implementation of the classic "Hunt the Wumpus" game, built using the MuddyCore framework's Properties extension system.

## Overview

Wumpus Hunt demonstrates advanced MuddyCore features including:
- **Properties-based Extensions**: Game logic without core framework modification
- **Lua Scripting**: AI behavior and game mechanics
- **Real-time Multiplayer**: Multiple players in shared and instanced worlds
- **Persistent Data**: Player authentication, statistics, and inventory
- **Instance Management**: Dynamic maze generation and cleanup
- **Comprehensive Error Handling**: Graceful recovery from corrupted game state

## Quick Start

### Prerequisites
- Go 1.21 or higher
- MuddyCore framework (automatically handled via go.mod)

### Running the Server
```bash
# Clone the repository and navigate to wumpus-hunt
cd examples/wumpus-hunt

# Run the server (default: localhost:8080)
go run . server

# Run with custom settings
go run . server --host 0.0.0.0 --port 8080 --enable-ws --ws-port 8081 --max-connections 100
```

### Connecting as a Client
```bash
# Connect to local server
go run . client --host localhost --port 8080

# Connect to WebSocket server
go run . client --host localhost --port 8081 --use-ws
```

## Game Flow

1. **Authentication**: Create account with username/password
2. **The Inn**: Safe social area with chat functionality
3. **The Portal**: Generate and enter random maze instances
4. **Hunt Phase**: Navigate maze, avoid hazards, find and kill the Wumpus
5. **Resolution**: Victory (wumpus pelt reward) or death (respawn in Inn)

## Architecture

### Properties Extension Pattern

This implementation showcases the Properties extension pattern - a powerful way to add game-specific functionality without modifying the core MuddyCore framework.

**Key Benefits:**
- ✅ No core framework modification required
- ✅ Backward compatibility maintained
- ✅ Game logic isolated and maintainable
- ✅ Pattern reusable for other games
- ✅ Type-safe wrapper functions

**Extension Points:**
- `Player.Properties["wumpus_auth"]` - Authentication data
- `Player.Properties["wumpus_stats"]` - Game statistics
- `Player.Properties["wumpus_state"]` - Current game state
- `Room.Properties["wumpus_room"]` - Room-specific game data
- `World.Properties["wumpus_world"]` - Instance management data
- `Item.Properties["wumpus_item"]` - Trophy and reward data

### System Components

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   The Inn       │    │   The Portal    │    │  Wumpus Mazes   │
│   (Persistent)  │<-->│   (Persistent)  │<-->│  (Temporary)    │
│                 │    │                 │    │                 │
│ - Player Chat   │    │ - Maze Gen      │    │ - Random 8-20   │
│ - Respawn Point │    │ - Transport     │    │ - Wumpus AI     │
│ - Character Mgmt│    │ - Instance Mgmt │    │ - Victory/Death │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Package Structure

- **`auth/`** - Player authentication using Properties extensions
- **`world/`** - World and room management with Properties
- **`items/`** - Item system and inventory management
- **`maze/`** - Random maze generation algorithms
- **`instance/`** - Temporary world instance management
- **`portal/`** - Transportation between worlds
- **`game/`** - Game state and combat mechanics
- **`ai/`** - Wumpus movement AI algorithms
- **`scripting/`** - Lua integration for game logic
- **`errors/`** - Comprehensive error handling and recovery
- **`performance/`** - Caching and optimization systems
- **`logging/`** - Game event logging and monitoring

## Testing

Run the comprehensive test suite:

```bash
# Run all tests with verbose output
go test ./... -v

# Run tests with coverage
go test ./... -cover

# Run specific package tests
go test ./auth -v
go test ./maze -v
```

The test suite includes:
- 200+ unit tests across all packages
- Integration tests for complete game flow
- Performance benchmarks
- Error recovery scenarios
- Multiplayer concurrency tests

## Development

### Adding New Features

1. **Identify Extension Point**: Determine which Properties field to use
2. **Create Wrapper Functions**: Add type-safe getters/setters
3. **Implement Game Logic**: Use existing MuddyCore systems
4. **Add Comprehensive Tests**: Unit, integration, and error tests
5. **Update Documentation**: Code comments and examples

### Properties Extension Guidelines

```go
// Safe reading pattern
func GetWumpusData(entity interface{}) (*WumpusData, error) {
    data, exists := entity.GetProperty("wumpus_key")
    if !exists {
        return nil, errors.New("no wumpus data found")
    }
    
    dataMap, ok := data.(map[string]interface{})
    if !ok {
        return nil, errors.New("invalid data format")
    }
    
    return parseWumpusData(dataMap)
}

// Safe writing pattern
func SetWumpusData(entity interface{}, data *WumpusData) error {
    dataMap := map[string]interface{}{
        "field1": data.Field1,
        "field2": data.Field2,
    }
    return entity.SetProperty("wumpus_key", dataMap)
}
```

## Performance

The implementation includes several performance optimizations:

- **Property Caching**: High-performance cache for frequently accessed Properties
- **Instance Pooling**: Reuse of world/room objects to reduce allocation
- **Background Cleanup**: Efficient removal of temporary data
- **Connection Management**: Optimized handling of multiple concurrent players

Benchmark results show excellent performance for 100+ concurrent players.

## Administration

### Server Configuration

```bash
# Basic server with logging
go run . server --host 0.0.0.0 --port 8080 --log-level debug

# Production server with monitoring
go run . server --host 0.0.0.0 --port 8080 --enable-ws --ws-port 8081 \
  --max-connections 500 --log-level info --enable-monitoring --metrics-port 9090
```

### Monitoring

Access monitoring dashboard at `http://localhost:9090/dashboard` when monitoring is enabled.

Key metrics:
- Active players and connections
- Game instances and cleanup statistics
- Properties access patterns and cache performance
- Error rates and recovery actions

### Backup and Recovery

Player data is automatically persisted using MuddyCore's storage system. For backup:

```bash
# Backup player data directory
cp -r portal/data/ backup/$(date +%Y%m%d)/

# The system includes automatic recovery for corrupted Properties
```

## License

This example is part of the MuddyCore project and follows the same license terms.

## Contributing

Contributions welcome! Please ensure:
1. All tests pass: `go test ./... -v`
2. Code follows Properties extension patterns
3. Comprehensive test coverage for new features
4. Documentation updates included

See [DESIGN.md](DESIGN.md) for detailed technical documentation.