package tetrapolruntime

import "time"

// Pipeline joins tetrapol_dump's frame output to RP-CELP. It remains external
// to the application binary, preserving each runtime's independent license.
type Pipeline struct {
	kit       Kit
	rpcelp    RPCELP
	telemetry telemetryTracker
}

type Config struct {
	KitExecutable    string
	KitArgs          []string
	RPCELPExecutable string
	RPCELPArgs       []string
	ClearOnly        bool
}

func (pipeline *Pipeline) Start(config Config, emit func([]float32)) error {
	pipeline.telemetry.reset()
	if err := pipeline.rpcelp.Start(config.RPCELPExecutable, config.RPCELPArgs, emit); err != nil {
		return err
	}
	if err := pipeline.kit.Start(config.KitExecutable, config.KitArgs, func(record []byte) {
		pipeline.telemetry.update(record, time.Now())
		if pipeline.telemetry.voiceAllowed(config.ClearOnly) {
			_, _ = pipeline.rpcelp.SubmitKitJSON(record)
		}
	}); err != nil {
		_ = pipeline.rpcelp.Stop()
		return err
	}
	return nil
}

func (pipeline *Pipeline) SubmitBits(bits []byte) error { return pipeline.kit.SubmitBits(bits) }

func (pipeline *Pipeline) Stop() error {
	kitErr := pipeline.kit.Stop()
	rpErr := pipeline.rpcelp.Stop()
	if kitErr != nil {
		return kitErr
	}
	return rpErr
}

func (pipeline *Pipeline) Running() bool { return pipeline.kit.Running() && pipeline.rpcelp.Running() }

func (pipeline *Pipeline) Telemetry() Telemetry { return pipeline.telemetry.snapshot(time.Now()) }

func (pipeline *Pipeline) String() string {
	if pipeline.Running() {
		return "TETRAPOL Kit + RP-CELP activo"
	}
	return "TETRAPOL runtime detenido"
}
