package storage

import (
	"fmt"
)

// ContainerValidator validates container operations
type ContainerValidator struct {
	itemManager *ItemManager
}

// NewContainerValidator creates a new container validator
func NewContainerValidator(itemManager *ItemManager) *ContainerValidator {
	return &ContainerValidator{
		itemManager: itemManager,
	}
}

// ContainerCalculation represents the result of container capacity calculations
type ContainerCalculation struct {
	TotalCapacity int
	UsedCapacity  int
	Available     int
	Unit          string
}

// CanFitInContainer checks if an item can fit in a container
func (cv *ContainerValidator) CanFitInContainer(containerInstance *ItemInstance, itemToAdd *ItemInstance, currentContents []*ItemInstance) (bool, error) {
	// Load container item definition
	containerItem, err := cv.itemManager.Load(containerInstance.ItemID)
	if err != nil {
		return false, fmt.Errorf("failed to load container item: %w", err)
	}

	// Check if it's actually a container
	if !containerItem.IsContainer {
		return false, fmt.Errorf("item %s is not a container", containerInstance.ItemID)
	}

	// Load item to add definition
	itemToAddDef, err := cv.itemManager.Load(itemToAdd.ItemID)
	if err != nil {
		return false, fmt.Errorf("failed to load item to add: %w", err)
	}

	// Check if item can be contained
	if !itemToAddDef.CanBeContained {
		return false, fmt.Errorf("item %s cannot be contained", itemToAdd.ItemID)
	}

	// Calculate capacity
	calculation, err := cv.CalculateContainerCapacity(containerInstance, currentContents)
	if err != nil {
		return false, err
	}

	// Calculate space needed for new item
	spaceNeeded := cv.calculateItemSpace(itemToAddDef, itemToAdd.Quantity, calculation.Unit)

	return spaceNeeded <= calculation.Available, nil
}

// CalculateContainerCapacity calculates the current capacity usage of a container
func (cv *ContainerValidator) CalculateContainerCapacity(containerInstance *ItemInstance, currentContents []*ItemInstance) (*ContainerCalculation, error) {
	// Load container item definition
	containerItem, err := cv.itemManager.Load(containerInstance.ItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to load container item: %w", err)
	}

	if !containerItem.IsContainer {
		return nil, fmt.Errorf("item %s is not a container", containerInstance.ItemID)
	}

	// Determine unit system
	unit := containerItem.ContainerSizeUnits
	if unit == "" {
		unit = "items"
	}

	calculation := &ContainerCalculation{
		TotalCapacity: containerItem.MaxContainerSize,
		UsedCapacity:  0,
		Unit:          unit,
	}

	// Calculate used capacity
	for _, instance := range currentContents {
		itemDef, err := cv.itemManager.Load(instance.ItemID)
		if err != nil {
			continue // Skip corrupted items
		}

		space := cv.calculateItemSpace(itemDef, instance.Quantity, unit)
		calculation.UsedCapacity += space
	}

	calculation.Available = calculation.TotalCapacity - calculation.UsedCapacity
	if calculation.Available < 0 {
		calculation.Available = 0
	}

	return calculation, nil
}

// calculateItemSpace calculates how much space an item takes based on unit system
func (cv *ContainerValidator) calculateItemSpace(item *Item, quantity int, unit string) int {
	switch unit {
	case "weight":
		return item.Weight * quantity
	case "volume":
		return item.Volume * quantity
	default: // "items" or unspecified
		return quantity
	}
}

// ValidateContainerNesting checks for infinite nesting loops
func (cv *ContainerValidator) ValidateContainerNesting(itemToAdd *ItemInstance, targetContainer *ItemInstance, allInstances []*ItemInstance) error {
	// If the item being added is not a container, no nesting issues
	itemDef, err := cv.itemManager.Load(itemToAdd.ItemID)
	if err != nil {
		return err
	}
	
	if !itemDef.IsContainer {
		return nil // Not a container, no nesting concerns
	}

	// Build a map of container relationships
	containerMap := make(map[string]string) // instanceID -> containerID
	for _, instance := range allInstances {
		if instance.ContainerID != "" {
			containerMap[instance.ID] = instance.ContainerID
		}
	}

	// Check if adding this item would create a loop
	visited := make(map[string]bool)
	return cv.checkForLoop(itemToAdd.ID, targetContainer.ID, containerMap, visited)
}

// checkForLoop recursively checks for container loops
func (cv *ContainerValidator) checkForLoop(startID, currentID string, containerMap map[string]string, visited map[string]bool) error {
	if visited[currentID] {
		return fmt.Errorf("container loop detected: item %s would create circular containment", startID)
	}

	visited[currentID] = true

	// Check if current container is contained in something else
	if parentID, exists := containerMap[currentID]; exists {
		if parentID == startID {
			return fmt.Errorf("container loop detected: item %s would contain itself", startID)
		}
		return cv.checkForLoop(startID, parentID, containerMap, visited)
	}

	return nil
}

// ContainerOperations provides high-level container operations
type ContainerOperations struct {
	itemManager       *ItemManager
	inventoryManager  *InventoryManager
	validator         *ContainerValidator
}

// NewContainerOperations creates a new container operations manager
func NewContainerOperations(itemManager *ItemManager, inventoryManager *InventoryManager) *ContainerOperations {
	return &ContainerOperations{
		itemManager:      itemManager,
		inventoryManager: inventoryManager,
		validator:        NewContainerValidator(itemManager),
	}
}

// PutItemInContainer moves an item into a container within an inventory
func (co *ContainerOperations) PutItemInContainer(inventory *Inventory, itemInstanceID, containerInstanceID string) error {
	// Prevent self-containment
	if itemInstanceID == containerInstanceID {
		return fmt.Errorf("item cannot contain itself")
	}

	// Find the item to move
	itemInstance := inventory.FindItem(itemInstanceID)
	if itemInstance == nil {
		return fmt.Errorf("item instance %s not found in inventory", itemInstanceID)
	}

	// Find the target container
	containerInstance := inventory.FindItem(containerInstanceID)
	if containerInstance == nil {
		return fmt.Errorf("container instance %s not found in inventory", containerInstanceID)
	}

	// Get current contents of the container
	currentContents := inventory.GetItemsInContainer(containerInstanceID)

	// Validate the operation
	canFit, err := co.validator.CanFitInContainer(containerInstance, itemInstance, currentContents)
	if err != nil {
		return fmt.Errorf("container validation failed: %w", err)
	}
	if !canFit {
		return fmt.Errorf("item %s cannot fit in container %s", itemInstanceID, containerInstanceID)
	}

	// Check for nesting loops
	if err := co.validator.ValidateContainerNesting(itemInstance, containerInstance, inventory.Items); err != nil {
		return fmt.Errorf("nesting validation failed: %w", err)
	}

	// Move the item
	return inventory.MoveItemToContainer(itemInstanceID, containerInstanceID)
}

// RemoveItemFromContainer removes an item from a container (moves it to top level)
func (co *ContainerOperations) RemoveItemFromContainer(inventory *Inventory, itemInstanceID string) error {
	// Find the item
	itemInstance := inventory.FindItem(itemInstanceID)
	if itemInstance == nil {
		return fmt.Errorf("item instance %s not found in inventory", itemInstanceID)
	}

	if itemInstance.ContainerID == "" {
		return fmt.Errorf("item %s is not in a container", itemInstanceID)
	}

	// Move to top level (empty container ID)
	return inventory.MoveItemToContainer(itemInstanceID, "")
}

// GetContainerContents returns all items directly contained in a container
func (co *ContainerOperations) GetContainerContents(inventory *Inventory, containerInstanceID string) ([]*ItemInstance, error) {
	// Verify container exists
	containerInstance := inventory.FindItem(containerInstanceID)
	if containerInstance == nil {
		return nil, fmt.Errorf("container instance %s not found", containerInstanceID)
	}

	// Verify it's actually a container
	containerItem, err := co.itemManager.Load(containerInstance.ItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to load container item: %w", err)
	}

	if !containerItem.IsContainer {
		return nil, fmt.Errorf("item %s is not a container", containerInstance.ItemID)
	}

	return inventory.GetItemsInContainer(containerInstanceID), nil
}

// GetContainerInfo returns detailed information about a container
func (co *ContainerOperations) GetContainerInfo(inventory *Inventory, containerInstanceID string) (*ContainerCalculation, error) {
	// Get container instance
	containerInstance := inventory.FindItem(containerInstanceID)
	if containerInstance == nil {
		return nil, fmt.Errorf("container instance %s not found", containerInstanceID)
	}

	// Get current contents
	currentContents := inventory.GetItemsInContainer(containerInstanceID)

	// Calculate capacity
	return co.validator.CalculateContainerCapacity(containerInstance, currentContents)
}

// GetAllNestedItems returns all items contained within a container (recursively)
func (co *ContainerOperations) GetAllNestedItems(inventory *Inventory, containerInstanceID string) ([]*ItemInstance, error) {
	var allItems []*ItemInstance
	
	// Get direct contents
	directContents := inventory.GetItemsInContainer(containerInstanceID)
	allItems = append(allItems, directContents...)

	// Recursively get contents of any containers within this container
	for _, item := range directContents {
		// Check if this item is also a container
		itemDef, err := co.itemManager.Load(item.ItemID)
		if err != nil {
			continue // Skip corrupted items
		}

		if itemDef.IsContainer {
			nested, err := co.GetAllNestedItems(inventory, item.ID)
			if err != nil {
				continue // Skip problematic nested containers
			}
			allItems = append(allItems, nested...)
		}
	}

	return allItems, nil
}

// CalculateTotalContainerWeight calculates the total weight of a container including all nested items
func (co *ContainerOperations) CalculateTotalContainerWeight(inventory *Inventory, containerInstanceID string) (int, error) {
	// Get the container item definition
	containerInstance := inventory.FindItem(containerInstanceID)
	if containerInstance == nil {
		return 0, fmt.Errorf("container instance %s not found", containerInstanceID)
	}

	containerItem, err := co.itemManager.Load(containerInstance.ItemID)
	if err != nil {
		return 0, fmt.Errorf("failed to load container item: %w", err)
	}

	totalWeight := containerItem.Weight * containerInstance.Quantity

	// Get all nested items and add their weights
	nestedItems, err := co.GetAllNestedItems(inventory, containerInstanceID)
	if err != nil {
		return totalWeight, err // Return container weight even if nested calculation fails
	}

	for _, item := range nestedItems {
		itemDef, err := co.itemManager.Load(item.ItemID)
		if err != nil {
			continue // Skip corrupted items
		}
		totalWeight += itemDef.Weight * item.Quantity
	}

	return totalWeight, nil
}