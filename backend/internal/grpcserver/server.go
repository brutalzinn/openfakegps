package grpcserver

import (
	"fmt"
	"io"
	"log"
	"sync"

	"google.golang.org/grpc"

	fakegpsv1 "github.com/openfakegps/openfakegps/backend/proto/fakegps/v1"

	"github.com/openfakegps/openfakegps/backend/internal/orchestration"
	"github.com/openfakegps/openfakegps/backend/internal/simulation"
)

// Server implements the AgentService gRPC server.
type Server struct {
	fakegpsv1.UnimplementedAgentServiceServer
	orchestrator *orchestration.Orchestrator
	registry     *orchestration.Registry
}

// NewServer creates a new gRPC server handler.
func NewServer(orch *orchestration.Orchestrator, reg *orchestration.Registry) *Server {
	return &Server{
		orchestrator: orch,
		registry:     reg,
	}
}

// Register attaches the service to a gRPC server.
func (s *Server) Register(gs *grpc.Server) {
	fakegpsv1.RegisterAgentServiceServer(gs, s)
}

// Connect handles the bidirectional streaming RPC from agents.
func (s *Server) Connect(stream fakegpsv1.AgentService_ConnectServer) error {
	// The first message must be a RegisterRequest.
	msg, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receiving first message: %w", err)
	}
	reg := msg.GetRegister()
	if reg == nil {
		return fmt.Errorf("first message must be a RegisterRequest")
	}

	agentID := reg.AgentId
	if agentID == "" {
		return fmt.Errorf("agent_id is required")
	}

	log.Printf("agent %s connecting: %s (%s)", agentID, reg.DeviceName, reg.DeviceModel)

	// Check if this is a reconnect.
	if _, ok := s.registry.Get(agentID); ok {
		s.orchestrator.HandleReconnect(agentID)
	}

	// Create the stream wrapper before registering, so it is available immediately.
	sw := newStreamWrapper(stream)

	// Register the device.
	s.registry.Register(orchestration.DeviceInfo{
		AgentID:      agentID,
		DeviceName:   reg.DeviceName,
		Model:        reg.DeviceModel,
		Capabilities: reg.Capabilities,
		Status:       orchestration.StatusIdle,
		Stream:       sw,
	})
	s.registry.SetStream(agentID, sw)

	// Send registration acceptance.
	if err := stream.Send(&fakegpsv1.ServerMessage{
		Payload: &fakegpsv1.ServerMessage_RegisterResponse{
			RegisterResponse: &fakegpsv1.RegisterResponse{
				Accepted: true,
				Message:  "registered",
			},
		},
	}); err != nil {
		return fmt.Errorf("sending register response: %w", err)
	}

	log.Printf("agent %s registered successfully", agentID)

	// Read loop: process heartbeats, status updates, etc.
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			log.Printf("agent %s disconnected (EOF)", agentID)
			break
		}
		if err != nil {
			log.Printf("agent %s stream error: %v", agentID, err)
			break
		}

		if msg.GetHeartbeat() != nil {
			s.registry.UpdateLastSeen(agentID)
		}

		if su := msg.GetStatusUpdate(); su != nil {
			log.Printf("agent %s status update: %v - %s",
				agentID, su.Status, su.Message)
		}
	}

	// Clean up on disconnect.
	s.orchestrator.HandleDisconnect(agentID)
	return nil
}

// streamWrapper implements orchestration.ServerStream by wrapping the gRPC stream.
type streamWrapper struct {
	stream fakegpsv1.AgentService_ConnectServer
	mu     sync.Mutex
}

func newStreamWrapper(stream fakegpsv1.AgentService_ConnectServer) *streamWrapper {
	return &streamWrapper{stream: stream}
}

// SendLocationUpdate sends a location update to the connected agent.
func (sw *streamWrapper) SendLocationUpdate(simID string, pos simulation.Position) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	return sw.stream.Send(&fakegpsv1.ServerMessage{
		Payload: &fakegpsv1.ServerMessage_LocationUpdate{
			LocationUpdate: &fakegpsv1.LocationUpdate{
				SimulationId: simID,
				Latitude:     pos.Lat,
				Longitude:    pos.Lon,
				Speed:        float32(pos.Speed),
				Bearing:      float32(pos.Bearing),
				Accuracy:     float32(pos.Accuracy),
				Altitude:     pos.Altitude,
				TimestampMs:  pos.Timestamp.UnixMilli(),
			},
		},
	})
}

// SendSimulationCommand sends a simulation command to the connected agent.
func (sw *streamWrapper) SendSimulationCommand(action string, simID string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	var protoAction fakegpsv1.SimulationAction
	switch action {
	case "start":
		protoAction = fakegpsv1.SimulationAction_SIMULATION_ACTION_START
	case "stop":
		protoAction = fakegpsv1.SimulationAction_SIMULATION_ACTION_STOP
	case "pause":
		protoAction = fakegpsv1.SimulationAction_SIMULATION_ACTION_PAUSE
	case "resume":
		protoAction = fakegpsv1.SimulationAction_SIMULATION_ACTION_RESUME
	default:
		protoAction = fakegpsv1.SimulationAction_SIMULATION_ACTION_UNSPECIFIED
	}

	return sw.stream.Send(&fakegpsv1.ServerMessage{
		Payload: &fakegpsv1.ServerMessage_SimulationCommand{
			SimulationCommand: &fakegpsv1.SimulationCommand{
				Action:       protoAction,
				SimulationId: simID,
			},
		},
	})
}
