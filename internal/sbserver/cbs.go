package sbserver

// CBS (claims-based security) handling. azservicebus authenticates by opening a
// link to the "$cbs" node and sending a put-token request. The emulator does
// not verify tokens; it replies 200 to every request so the SDK proceeds.

func (c *conn) handleCBS(reqLink *link, payload []byte) error {
	messageID := parseMessageID(payload)
	response := buildCBSResponse(messageID)

	receiver := c.findCBSReceiver(reqLink.channel)
	if receiver == nil {
		return nil
	}
	return c.deliver(receiver, response)
}

// findCBSReceiver returns the link on the given session that the client uses to
// receive CBS responses.
func (c *conn) findCBSReceiver(channel uint16) *link {
	c.linksMu.Lock()
	defer c.linksMu.Unlock()
	for _, l := range c.links {
		if l.isCBS && !l.clientIsSender && l.channel == channel {
			return l
		}
	}
	return nil
}

// parseMessageID extracts properties.message-id (field 0) from an encoded
// message, used to correlate the CBS response.
func parseMessageID(payload []byte) any {
	dec := newDecoder(payload)
	for dec.remaining() > 0 {
		v, err := dec.readValue()
		if err != nil {
			return nil
		}
		desc, ok := v.(described)
		if !ok {
			continue
		}
		if descriptorCode(desc.descriptor) != descProperties {
			continue
		}
		fields, ok := desc.value.([]any)
		if !ok || len(fields) == 0 {
			return nil
		}
		return fields[0]
	}
	return nil
}

// buildCBSResponse encodes a message carrying correlation-id and a 200 status
// in its application properties.
func buildCBSResponse(correlationID any) []byte {
	out := make([]byte, 0, 64)

	// properties: correlation-id is field index 5.
	out = append(out, encodeDescribedList(descProperties, []any{
		nil, // message-id
		nil, // user-id
		nil, // to
		nil, // subject
		nil, // reply-to
		correlationID,
	})...)

	// application-properties: a map with the status code.
	e := newEncoder()
	e.writeValue(described{
		descriptor: uint64(descAppProps),
		value: map[any]any{
			"status-code":        int32(200),
			"status-description": "OK",
		},
	})
	out = append(out, e.bytes()...)
	return out
}
