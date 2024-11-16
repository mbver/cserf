package serf

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
)

const (
	installKey          = "install-key"
	useKey              = "use-key"
	removeKey           = "remove-key"
	listKey             = "list-key"
	minEncodedKeyLength = 25
)

type KeyQueryResponse struct {
	NumNode         int
	NumResp         int
	NumErr          int
	ErrFrom         map[string]string
	PrimaryKeyCount map[string]int
	KeyCount        map[string]int
}

type KeyResponse struct {
	Success    bool
	Err        string
	Keys       []string // for list key
	PrimaryKey string   // for list key
}

func isKeyCommand(command string) bool {
	return command == "install" || command == "use" ||
		command == "remove" || command == "list"
}

func keyCommandToQueryName(command string) string {
	return command + "-key"
}

func isValidKeyQueryName(name string) bool {
	return name == installKey || name == useKey ||
		name == removeKey || name == listKey
}

func (s *Serf) KeyQuery(command string, key string) (*KeyQueryResponse, error) {
	if !isKeyCommand(command) {
		return nil, fmt.Errorf("not a key command %s", command)
	}
	payload, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}
	params := s.DefaultQueryParams()
	params.Name = keyCommandToQueryName(command)
	params.Payload = payload

	kResp := &KeyQueryResponse{
		NumNode:         s.mlist.NumActive(),
		ErrFrom:         make(map[string]string),
		PrimaryKeyCount: make(map[string]int),
		KeyCount:        make(map[string]int),
	}
	respCh := make(chan *QueryResponse)
	if err := s.Query(respCh, params); err != nil {
		return nil, err
	}
	collectKeyResponse(kResp, respCh)
	return kResp, nil
}

func collectKeyResponse(kr *KeyQueryResponse, respCh chan *QueryResponse) {
	for r := range respCh {
		if kr.NumResp == kr.NumNode {
			return
		}
		kr.NumResp++
		if len(r.Payload) == 0 || r.Payload[0] != byte(msgKeyRespType) {
			kr.ErrFrom[r.From] = "query response doesn't contain key response"
			kr.NumErr++
			continue
		}
		var resp KeyResponse
		if err := decode(r.Payload[1:], &resp); err != nil {
			kr.ErrFrom[r.From] = "fail to decode query response"
			kr.NumErr++
			continue
		}
		if !resp.Success {
			kr.NumErr++
		}
		if len(resp.Err) > 0 {
			kr.ErrFrom[r.From] = resp.Err
		}
		// for list keys
		for _, key := range resp.Keys {
			kr.KeyCount[key]++
		}
		if len(resp.PrimaryKey) != 0 {
			kr.PrimaryKeyCount[resp.PrimaryKey]++
		}

	}
}

type keyQueryReceptor struct {
	inCh  chan Event
	outCh chan Event
}

func (s *Serf) receiveKeyEvents() {
	for {
		select {
		case e := <-s.keyQuery.inCh:
			q, ok := e.(*QueryEvent)
			if !ok || !isValidKeyQueryName(q.Name) {
				s.keyQuery.outCh <- e
				continue
			}
			s.handleKeyQuery(q)
		case <-s.shutdownCh:
			s.logger.Printf("[WARN] serf: serf shutdown, quitting receiving events")
			return
		}
	}
}

func (s *Serf) sendKeyResponse(q *QueryEvent, resp *KeyResponse) error {
	encoded, err := encode(msgKeyRespType, resp)
	if err != nil {
		s.logger.Printf("[ERR] serf: failed to encode key response: %v", err)
		return err
	}
	return s.respondToQueryEvent(q, encoded)
}

func (s *Serf) handleKeyQuery(q *QueryEvent) {

	s.logger.Printf("[DEBUG] serf: Received %s query", q.Name)

	if !s.mlist.EncryptionEnabled() {
		s.logger.Printf("[ERR] serf: encryption not enabled")
		s.sendKeyResponse(q, &KeyResponse{Err: "encryption not enabled"})
		return
	}

	if q.Name == "list-key" {
		s.handleListKey(q)
		return
	}

	resp := &KeyResponse{Success: false}
	var err error
	defer func() {
		if err != nil {
			s.logger.Printf("[ERR] serf: %s", err)
			resp.Err = err.Error()
		}
		s.sendKeyResponse(q, resp)
	}()

	key := q.Payload

	switch q.Name {
	case installKey:
		if err = s.keyring.AddKey(key); err != nil {
			return
		}
	case useKey:
		if err = s.keyring.UseKey(key); err != nil {
			return
		}
	case removeKey:
		if err = s.keyring.RemoveKey(key); err != nil {
			return
		}
	default:
		err = fmt.Errorf("unknown key query %s", q.Name)
		return
	}
	if err = s.writeKeyringFile(); err != nil {
		return
	}
	resp.Success = true

}

func (s *Serf) writeKeyringFile() error {
	if s.config.KeyringFile == "" {
		return nil
	}
	keys := s.keyring.GetKeys()
	encoded := make([]string, len(keys))

	for i, key := range keys {
		encoded[i] = base64.StdEncoding.EncodeToString(key)
	}
	jsonEncoded, err := json.MarshalIndent(encoded, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode keys: %s", err)
	}
	// Use 0600 for permissions because key data is sensitive
	if err = os.WriteFile(s.config.KeyringFile, jsonEncoded, 0600); err != nil {
		return fmt.Errorf("failed to write keyring file: %s", err)
	}
	// Success!
	return nil
}

func (s *Serf) handleListKey(q *QueryEvent) {
	resp := &KeyResponse{}
	pKey := s.keyring.GetPrimaryKey()
	resp.PrimaryKey = base64.StdEncoding.EncodeToString(pKey)

	for _, key := range s.keyring.GetKeys() {
		encoded := base64.StdEncoding.EncodeToString(key)
		resp.Keys = append(resp.Keys, encoded)
	}
	resp.Success = true
	s.trySendlistKeyResponse(q, resp)
}

func (s *Serf) trySendlistKeyResponse(q *QueryEvent, r *KeyResponse) {
	nKeys := s.config.QueryResponseSizeLimit / minEncodedKeyLength
	if nKeys > len(r.Keys) {
		nKeys = len(r.Keys)
	}
	for i := nKeys; i > 0; i-- {
		r.Keys = r.Keys[:i]
		if i < len(r.Keys) {
			r.Err = "key-list truncated"
		}
		err := s.sendKeyResponse(q, r)
		if err == ErrQueryRespLimitExceed {
			continue
		}
		if err != nil {
			s.logger.Printf("[ERR] serf: failed to send list key respose %v", err)
		}
		return
	}
	s.logger.Printf("[ERR] serf: unable to send list-key response")
}
