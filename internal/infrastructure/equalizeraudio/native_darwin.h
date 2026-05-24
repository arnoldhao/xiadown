//go:build darwin && cgo && !ios

#ifndef XIADOWN_EQUALIZER_NATIVE_DARWIN_H
#define XIADOWN_EQUALIZER_NATIVE_DARWIN_H

#ifdef __cplusplus
extern "C" {
#endif

enum {
	XiaEQStartSuccess = 0,
	XiaEQStartUnsupported = 1,
	XiaEQStartPermissionDenied = 2,
	XiaEQStartNoAudioSource = 3,
	XiaEQStartTapCreation = 4,
	XiaEQStartAggregateCreation = 5,
	XiaEQStartInvalidTapFormat = 6,
	XiaEQStartIOProcInstall = 7,
	XiaEQStartDeviceStart = 8,
};

int xia_equalizer_supported(void);
int xia_equalizer_has_capture_permission(void);
int xia_equalizer_request_capture_permission(void);
int xia_equalizer_is_running(void);
int xia_equalizer_has_observed_audio(void);
int xia_equalizer_visualizer_frame(
	float *bands,
	int bandCount,
	float *waveform,
	int waveformCount,
	double *level,
	unsigned long long *sequence,
	double *analysisTimeSeconds,
	double *frameTimeOffsetSeconds
);
int xia_equalizer_start(int enabled, double preampDB, const double *gains, int gainCount, int *detailStatus);
void xia_equalizer_apply(int enabled, double preampDB, const double *gains, int gainCount);
void xia_equalizer_stop(void);

#ifdef __cplusplus
}
#endif

#endif
