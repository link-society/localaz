package tablestore

import "sort"

// sortEntities orders entities by partition key then row key, the natural order
// Azure Table Storage returns rows in.
func sortEntities(entities []*Entity) {
	sort.Slice(entities, func(i, j int) bool {
		if entities[i].PartitionKey != entities[j].PartitionKey {
			return entities[i].PartitionKey < entities[j].PartitionKey
		}
		return entities[i].RowKey < entities[j].RowKey
	})
}
