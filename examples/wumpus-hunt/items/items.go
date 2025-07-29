package items

import (
	"errors"
	"fmt"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// WumpusItemType represents the type of wumpus-related item
type WumpusItemType string

const (
	WumpusItemTypeTrophy WumpusItemType = "trophy"
	WumpusItemTypeWeapon WumpusItemType = "weapon"
	WumpusItemTypeArmor  WumpusItemType = "armor"
)

// WumpusItemRarity represents the rarity of wumpus-related items
type WumpusItemRarity string

const (
	WumpusItemRarityCommon    WumpusItemRarity = "common"
	WumpusItemRarityUncommon  WumpusItemRarity = "uncommon"
	WumpusItemRarityRare      WumpusItemRarity = "rare"
	WumpusItemRarityLegendary WumpusItemRarity = "legendary"
)

// WumpusItemData represents game-specific data for wumpus items
type WumpusItemData struct {
	ItemType           WumpusItemType   `json:"item_type"`
	Rarity             WumpusItemRarity `json:"rarity"`
	Description        string           `json:"description"`
	VictoryTimestamp   string           `json:"victory_timestamp,omitempty"`
	DefeatedInInstance string           `json:"defeated_in_instance,omitempty"`
	PlayerName         string           `json:"player_name,omitempty"`
	DamageBonus        int              `json:"damage_bonus,omitempty"`
	DefenseBonus       int              `json:"defense_bonus,omitempty"`
}

// CreateWumpusPelt creates a new wumpus pelt trophy item
func CreateWumpusPelt(instanceID, playerName string) *storage.Item {
	item := storage.NewItem(
		ids.NewEntityID(),
		"Wumpus Pelt",
	)
	item.Description = "A dark, thick pelt taken from a defeated wumpus. The fur is coarse and smells of ancient caves."

	// Set wumpus-specific properties
	wumpusData := WumpusItemData{
		ItemType:           WumpusItemTypeTrophy,
		Rarity:             WumpusItemRarityRare,
		Description:        "A trophy from defeating the mighty wumpus in the depths of the maze",
		VictoryTimestamp:   time.Now().Format(time.RFC3339),
		DefeatedInInstance: instanceID,
		PlayerName:         playerName,
	}

	item.SetProperty("wumpus_item", wumpusData)
	return item
}

// CreateWumpusSpear creates a basic hunting spear
func CreateWumpusSpear() *storage.Item {
	item := storage.NewItem(
		ids.NewEntityID(),
		"Hunting Spear",
	)
	item.Description = "A sturdy wooden spear with a sharp iron tip, perfect for hunting dangerous beasts."

	wumpusData := WumpusItemData{
		ItemType:     WumpusItemTypeWeapon,
		Rarity:       WumpusItemRarityCommon,
		Description:  "A reliable weapon for wumpus hunting",
		DamageBonus:  10,
	}

	item.SetProperty("wumpus_item", wumpusData)
	return item
}

// CreateWumpusArmor creates basic leather armor
func CreateWumpusArmor() *storage.Item {
	item := storage.NewItem(
		ids.NewEntityID(),
		"Leather Armor",
	)
	item.Description = "Thick leather armor that provides some protection against claws and teeth."

	wumpusData := WumpusItemData{
		ItemType:     WumpusItemTypeArmor,
		Rarity:       WumpusItemRarityCommon,
		Description:  "Basic protection for wumpus hunters",
		DefenseBonus: 5,
	}

	item.SetProperty("wumpus_item", wumpusData)
	return item
}

// GetWumpusItemData retrieves wumpus-specific data from an item
func GetWumpusItemData(item *storage.Item) (*WumpusItemData, error) {
	itemData, exists := item.GetProperty("wumpus_item")
	if !exists {
		return nil, errors.New("item does not have wumpus data")
	}

	// Handle both direct struct and map[string]interface{} cases
	switch v := itemData.(type) {
	case WumpusItemData:
		return &v, nil
	case map[string]interface{}:
		data := &WumpusItemData{}
		
		if itemType, ok := v["item_type"].(string); ok {
			data.ItemType = WumpusItemType(itemType)
		}
		if rarity, ok := v["rarity"].(string); ok {
			data.Rarity = WumpusItemRarity(rarity)
		}
		if description, ok := v["description"].(string); ok {
			data.Description = description
		}
		if timestamp, ok := v["victory_timestamp"].(string); ok {
			data.VictoryTimestamp = timestamp
		}
		if instance, ok := v["defeated_in_instance"].(string); ok {
			data.DefeatedInInstance = instance
		}
		if playerName, ok := v["player_name"].(string); ok {
			data.PlayerName = playerName
		}
		if damageBonus, ok := v["damage_bonus"].(float64); ok {
			data.DamageBonus = int(damageBonus)
		}
		if defenseBonus, ok := v["defense_bonus"].(float64); ok {
			data.DefenseBonus = int(defenseBonus)
		}
		
		return data, nil
	default:
		return nil, errors.New("invalid wumpus item data format")
	}
}

// IsWumpusItem checks if an item is a wumpus-related item
func IsWumpusItem(item *storage.Item) bool {
	_, err := GetWumpusItemData(item)
	return err == nil
}

// IsWumpusPelt checks if an item is specifically a wumpus pelt
func IsWumpusPelt(item *storage.Item) bool {
	data, err := GetWumpusItemData(item)
	if err != nil {
		return false
	}
	return data.ItemType == WumpusItemTypeTrophy
}

// GivePlayerItem gives an item to a player by creating an ItemInstance
func GivePlayerItem(player *storage.Player, item *storage.Item, quantity int) *storage.ItemInstance {
	instance := storage.NewItemInstance(
		ids.NewEntityID(),
		item.ID,
		quantity,
	)
	
	// Copy item properties to instance for persistence
	if wumpusData, err := GetWumpusItemData(item); err == nil {
		instance.Properties["wumpus_item"] = *wumpusData
	}
	
	return instance
}

// GetPlayerWumpusPelts returns all wumpus pelt instances owned by a player
func GetPlayerWumpusPelts(player *storage.Player, storageMgr *storage.StorageManager) ([]*storage.ItemInstance, error) {
	// This would typically query the storage manager for player's inventory
	// For now, we'll return a placeholder implementation
	// In a real implementation, this would:
	// 1. Query player's inventory from storage
	// 2. Filter for ItemInstances that have wumpus_item properties
	// 3. Further filter for trophy type items
	
	// TODO: Implement actual inventory querying when storage supports it
	return []*storage.ItemInstance{}, nil
}

// GetPlayerWumpusPeltCount returns the number of wumpus pelts a player has
func GetPlayerWumpusPeltCount(player *storage.Player, storageMgr *storage.StorageManager) (int, error) {
	pelts, err := GetPlayerWumpusPelts(player, storageMgr)
	if err != nil {
		return 0, err
	}
	
	count := 0
	for _, pelt := range pelts {
		count += pelt.Quantity
	}
	
	return count, nil
}

// FormatItemDescription returns a formatted description of a wumpus item
func FormatItemDescription(item *storage.Item) string {
	data, err := GetWumpusItemData(item)
	if err != nil {
		return item.Description
	}
	
	description := fmt.Sprintf("%s\n\n%s", item.Description, data.Description)
	
	if data.ItemType == WumpusItemTypeTrophy && data.VictoryTimestamp != "" {
		if timestamp, err := time.Parse(time.RFC3339, data.VictoryTimestamp); err == nil {
			description += fmt.Sprintf("\n\nDefeated on: %s", timestamp.Format("January 2, 2006 at 15:04"))
		}
		if data.PlayerName != "" {
			description += fmt.Sprintf("\nVictorious Hunter: %s", data.PlayerName)
		}
	}
	
	if data.DamageBonus > 0 {
		description += fmt.Sprintf("\nDamage Bonus: +%d", data.DamageBonus)
	}
	
	if data.DefenseBonus > 0 {
		description += fmt.Sprintf("\nDefense Bonus: +%d", data.DefenseBonus)
	}
	
	return description
}

// GetRarityDisplayName returns a display-friendly name for item rarity
func GetRarityDisplayName(rarity WumpusItemRarity) string {
	switch rarity {
	case WumpusItemRarityCommon:
		return "Common"
	case WumpusItemRarityUncommon:
		return "Uncommon"
	case WumpusItemRarityRare:
		return "Rare"
	case WumpusItemRarityLegendary:
		return "Legendary"
	default:
		return "Unknown"
	}
}

// GetTypeDisplayName returns a display-friendly name for item type
func GetTypeDisplayName(itemType WumpusItemType) string {
	switch itemType {
	case WumpusItemTypeTrophy:
		return "Trophy"
	case WumpusItemTypeWeapon:
		return "Weapon"
	case WumpusItemTypeArmor:
		return "Armor"
	default:
		return "Unknown"
	}
}