//go:build darwin && cgo && !ios

#import "native_darwin.h"

#import <ApplicationServices/ApplicationServices.h>
#import <AppKit/AppKit.h>
#import <CoreAudio/AudioHardware.h>
#import <CoreAudio/AudioHardwareTapping.h>
#import <CoreAudio/CATapDescription.h>
#import <CoreAudio/HostTime.h>
#import <CoreGraphics/CoreGraphics.h>
#import <Foundation/Foundation.h>
#import <WebKit/WebKit.h>
#include <dispatch/dispatch.h>
#include <libproc.h>
#include <math.h>
#include <objc/message.h>
#include <pthread.h>
#include <stdint.h>
#include <string.h>
#include <sys/types.h>

typedef struct {
	double b0;
	double b1;
	double b2;
	double a1;
	double a2;
	double targetB0;
	double targetB1;
	double targetB2;
	double targetA1;
	double targetA2;
	double leftZ1;
	double leftZ2;
	double rightZ1;
	double rightZ2;
} XiaBiquad;

typedef struct {
	AudioObjectID tapID;
	AudioObjectID aggregateID;
	AudioDeviceIOProcID ioProcID;
	CFStringRef outputDeviceUID;
	Float64 sampleRate;
	int running;
	int defaultOutputListenerInstalled;
	int hasObservedAudio;
	int settingsEnabled;
	double settingsPreampDB;
	double settingsGainsDB[6];
	float preampLinear;
	float wetMix;
	float wetMixTarget;
	float envStereo;
	float envMono;
	float limiterGainStereo;
	float limiterGainMono;
	XiaBiquad filters[6];
} XiaEqualizerEngine;

typedef struct {
	float bands[32];
	float waveform[64];
	float level;
	double analysisTimeSeconds;
	double frameHostTimeSeconds;
	uint64_t sequence;
	int running;
} XiaVisualizerFrame;

static pthread_mutex_t xiaEngineLock = PTHREAD_MUTEX_INITIALIZER;
static pthread_mutex_t xiaVisualizerLock = PTHREAD_MUTEX_INITIALIZER;
static XiaEqualizerEngine xiaEngine;
static XiaVisualizerFrame xiaVisualizerFrame;
static int xiaEngineInitialized = 0;
static dispatch_once_t xiaOutputRouteQueueOnce;
static dispatch_queue_t xiaOutputRouteQueueRef = NULL;
static volatile int xiaOutputRouteRestartScheduled = 0;
static double xiaAnalyzerSampleRate = 0;
static double xiaAnalyzerCoefficients[32];

static const char *xiaTapName = "com.dreamapp.XiaDown.EQ.Tap";
static const char *xiaAggregateName = "XiaDown EQ Aggregate";
static const char *xiaAggregateUIDPrefix = "com.dreamapp.XiaDown.EQ.Aggregate.";
static const int xiaBandCount = 6;
static const int xiaVisualizerBandCount = 32;
static const int xiaVisualizerWaveformCount = 64;
static const double xiaBandFrequencies[6] = {60, 150, 400, 1000, 2400, 15000};

static dispatch_queue_t xiaOutputRouteQueue(void) {
	dispatch_once(&xiaOutputRouteQueueOnce, ^{
		xiaOutputRouteQueueRef = dispatch_queue_create("com.dreamapp.XiaDown.equalizer.output-route", DISPATCH_QUEUE_SERIAL);
	});
	return xiaOutputRouteQueueRef;
}

static OSStatus xiaDefaultOutputDeviceChanged(
	AudioObjectID inObjectID,
	UInt32 inNumberAddresses,
	const AudioObjectPropertyAddress *inAddresses,
	void *inClientData
);
static void xiaRestartForDefaultOutputChange(void);

static double xiaCurrentHostTimeSeconds(void) {
	UInt64 hostTime = AudioGetCurrentHostTime();
	UInt64 nanos = AudioConvertHostTimeToNanos(hostTime);
	return (double)nanos / 1000000000.0;
}

static double xiaAudioFrameHostTimeSeconds(const AudioTimeStamp *timestamp, double durationSeconds) {
	if (
		timestamp != NULL &&
		(timestamp->mFlags & kAudioTimeStampHostTimeValid) != 0 &&
		timestamp->mHostTime > 0
	) {
		UInt64 nanos = AudioConvertHostTimeToNanos(timestamp->mHostTime);
		return ((double)nanos / 1000000000.0) + fmax(0, durationSeconds);
	}
	return xiaCurrentHostTimeSeconds();
}

static const double xiaBandQ[6] = {0.71, 0.55, 0.5, 0.5, 0.55, 0.71};

static NSString *xiaAudioKey(const char *key) {
	return [NSString stringWithUTF8String:key];
}

static void xiaBiquadSetUnity(XiaBiquad *filter) {
	if (filter == NULL) {
		return;
	}
	filter->b0 = 1;
	filter->b1 = 0;
	filter->b2 = 0;
	filter->a1 = 0;
	filter->a2 = 0;
	filter->targetB0 = 1;
	filter->targetB1 = 0;
	filter->targetB2 = 0;
	filter->targetA1 = 0;
	filter->targetA2 = 0;
	filter->leftZ1 = 0;
	filter->leftZ2 = 0;
	filter->rightZ1 = 0;
	filter->rightZ2 = 0;
}

static void xiaEnsureEngineInitialized(void) {
	if (xiaEngineInitialized) {
		return;
	}
	xiaEngine.tapID = kAudioObjectUnknown;
	xiaEngine.aggregateID = kAudioObjectUnknown;
	xiaEngine.ioProcID = NULL;
	xiaEngine.outputDeviceUID = NULL;
	xiaEngine.sampleRate = 48000;
	xiaEngine.running = 0;
	xiaEngine.defaultOutputListenerInstalled = 0;
	xiaEngine.hasObservedAudio = 0;
	xiaEngine.settingsEnabled = 0;
	xiaEngine.settingsPreampDB = 0;
	for (int index = 0; index < xiaBandCount; index++) {
		xiaEngine.settingsGainsDB[index] = 0;
	}
	xiaEngine.preampLinear = 1;
	xiaEngine.wetMix = 1;
	xiaEngine.wetMixTarget = 1;
	xiaEngine.envStereo = 0;
	xiaEngine.envMono = 0;
	xiaEngine.limiterGainStereo = 1;
	xiaEngine.limiterGainMono = 1;
	for (int index = 0; index < xiaBandCount; index++) {
		xiaBiquadSetUnity(&xiaEngine.filters[index]);
	}
	xiaEngineInitialized = 1;
}

static void xiaResetDSPState(void) {
	xiaEngine.hasObservedAudio = 0;
	xiaEngine.wetMix = xiaEngine.wetMixTarget;
	xiaEngine.envStereo = 0;
	xiaEngine.envMono = 0;
	xiaEngine.limiterGainStereo = 1;
	xiaEngine.limiterGainMono = 1;
	for (int index = 0; index < xiaBandCount; index++) {
		xiaEngine.filters[index].leftZ1 = 0;
		xiaEngine.filters[index].leftZ2 = 0;
		xiaEngine.filters[index].rightZ1 = 0;
		xiaEngine.filters[index].rightZ2 = 0;
	}
}

static int xiaNormalizeCoefficients(XiaBiquad *filter, double b0, double b1, double b2, double a0, double a1, double a2) {
	if (filter == NULL || !isfinite(a0) || fabs(a0) <= 1e-10) {
		return 0;
	}
	double invA0 = 1.0 / a0;
	filter->targetB0 = b0 * invA0;
	filter->targetB1 = b1 * invA0;
	filter->targetB2 = b2 * invA0;
	filter->targetA1 = a1 * invA0;
	filter->targetA2 = a2 * invA0;
	return 1;
}

static void xiaSetPeakingEQ(XiaBiquad *filter, double frequency, double q, double gainDB, double sampleRate) {
	if (sampleRate <= 0 || frequency <= 0 || q <= 0) {
		return;
	}
	double omega = 2.0 * M_PI * frequency / sampleRate;
	double sinOmega = sin(omega);
	double cosOmega = cos(omega);
	double alpha = sinOmega / (2.0 * q);
	double A = pow(10.0, gainDB / 40.0);
	xiaNormalizeCoefficients(
		filter,
		1.0 + alpha * A,
		-2.0 * cosOmega,
		1.0 - alpha * A,
		1.0 + alpha / A,
		-2.0 * cosOmega,
		1.0 - alpha / A
	);
}

static void xiaSetShelf(XiaBiquad *filter, int highShelf, double frequency, double slope, double gainDB, double sampleRate) {
	if (sampleRate <= 0 || frequency <= 0 || slope <= 0) {
		return;
	}
	double safeSlope = fmin(slope, 1.0);
	double omega = 2.0 * M_PI * frequency / sampleRate;
	double sinOmega = sin(omega);
	double cosOmega = cos(omega);
	double A = pow(10.0, gainDB / 40.0);
	double sqrtA = sqrt(A);
	double alpha = sinOmega / 2.0 * sqrt((A + 1.0 / A) * (1.0 / safeSlope - 1.0) + 2.0);
	double sign = highShelf ? -1.0 : 1.0;
	double aPlus1 = A + 1.0;
	double aMinus1 = A - 1.0;
	double twoSqrtAAlpha = 2.0 * sqrtA * alpha;
	xiaNormalizeCoefficients(
		filter,
		A * (aPlus1 - sign * aMinus1 * cosOmega + twoSqrtAAlpha),
		2.0 * A * (sign * aMinus1 - aPlus1 * cosOmega),
		A * (aPlus1 - sign * aMinus1 * cosOmega - twoSqrtAAlpha),
		aPlus1 + sign * aMinus1 * cosOmega + twoSqrtAAlpha,
		-2.0 * (sign * aMinus1 + aPlus1 * cosOmega),
		aPlus1 + sign * aMinus1 * cosOmega - twoSqrtAAlpha
	);
}

static void xiaProcessBiquadStereo(XiaBiquad *filter, float *left, float *right, int frameCount) {
	if (filter == NULL || left == NULL || right == NULL || frameCount <= 0) {
		return;
	}
	double b0 = filter->b0;
	double b1 = filter->b1;
	double b2 = filter->b2;
	double a1 = filter->a1;
	double a2 = filter->a2;
	double tb0 = filter->targetB0;
	double tb1 = filter->targetB1;
	double tb2 = filter->targetB2;
	double ta1 = filter->targetA1;
	double ta2 = filter->targetA2;
	double lz1 = filter->leftZ1;
	double lz2 = filter->leftZ2;
	double rz1 = filter->rightZ1;
	double rz2 = filter->rightZ2;
	const double alpha = 0.004;

	for (int index = 0; index < frameCount; index++) {
		b0 += (tb0 - b0) * alpha;
		b1 += (tb1 - b1) * alpha;
		b2 += (tb2 - b2) * alpha;
		a1 += (ta1 - a1) * alpha;
		a2 += (ta2 - a2) * alpha;

		double xLeft = left[index];
		double yLeft = b0 * xLeft + lz1;
		lz1 = b1 * xLeft - a1 * yLeft + lz2;
		lz2 = b2 * xLeft - a2 * yLeft;
		left[index] = (float)yLeft;

		double xRight = right[index];
		double yRight = b0 * xRight + rz1;
		rz1 = b1 * xRight - a1 * yRight + rz2;
		rz2 = b2 * xRight - a2 * yRight;
		right[index] = (float)yRight;
	}

	filter->b0 = b0;
	filter->b1 = b1;
	filter->b2 = b2;
	filter->a1 = a1;
	filter->a2 = a2;
	filter->leftZ1 = lz1;
	filter->leftZ2 = lz2;
	filter->rightZ1 = rz1;
	filter->rightZ2 = rz2;
}

static void xiaProcessBiquadInterleavedStereo(XiaBiquad *filter, float *samples, int frameCount, UInt32 channels) {
	if (filter == NULL || samples == NULL || frameCount <= 0 || channels < 2) {
		return;
	}
	double b0 = filter->b0;
	double b1 = filter->b1;
	double b2 = filter->b2;
	double a1 = filter->a1;
	double a2 = filter->a2;
	double tb0 = filter->targetB0;
	double tb1 = filter->targetB1;
	double tb2 = filter->targetB2;
	double ta1 = filter->targetA1;
	double ta2 = filter->targetA2;
	double lz1 = filter->leftZ1;
	double lz2 = filter->leftZ2;
	double rz1 = filter->rightZ1;
	double rz2 = filter->rightZ2;
	const double alpha = 0.004;

	for (int frame = 0; frame < frameCount; frame++) {
		b0 += (tb0 - b0) * alpha;
		b1 += (tb1 - b1) * alpha;
		b2 += (tb2 - b2) * alpha;
		a1 += (ta1 - a1) * alpha;
		a2 += (ta2 - a2) * alpha;

		UInt32 offset = (UInt32)frame * channels;
		double xLeft = samples[offset];
		double yLeft = b0 * xLeft + lz1;
		lz1 = b1 * xLeft - a1 * yLeft + lz2;
		lz2 = b2 * xLeft - a2 * yLeft;
		samples[offset] = (float)yLeft;

		double xRight = samples[offset + 1];
		double yRight = b0 * xRight + rz1;
		rz1 = b1 * xRight - a1 * yRight + rz2;
		rz2 = b2 * xRight - a2 * yRight;
		samples[offset + 1] = (float)yRight;
	}

	filter->b0 = b0;
	filter->b1 = b1;
	filter->b2 = b2;
	filter->a1 = a1;
	filter->a2 = a2;
	filter->leftZ1 = lz1;
	filter->leftZ2 = lz2;
	filter->rightZ1 = rz1;
	filter->rightZ2 = rz2;
}

static void xiaProcessBiquadMono(XiaBiquad *filter, float *samples, int frameCount) {
	if (filter == NULL || samples == NULL || frameCount <= 0) {
		return;
	}
	double b0 = filter->b0;
	double b1 = filter->b1;
	double b2 = filter->b2;
	double a1 = filter->a1;
	double a2 = filter->a2;
	double tb0 = filter->targetB0;
	double tb1 = filter->targetB1;
	double tb2 = filter->targetB2;
	double ta1 = filter->targetA1;
	double ta2 = filter->targetA2;
	double z1 = filter->leftZ1;
	double z2 = filter->leftZ2;
	const double alpha = 0.004;

	for (int index = 0; index < frameCount; index++) {
		b0 += (tb0 - b0) * alpha;
		b1 += (tb1 - b1) * alpha;
		b2 += (tb2 - b2) * alpha;
		a1 += (ta1 - a1) * alpha;
		a2 += (ta2 - a2) * alpha;

		double input = samples[index];
		double output = b0 * input + z1;
		z1 = b1 * input - a1 * output + z2;
		z2 = b2 * input - a2 * output;
		samples[index] = (float)output;
	}

	filter->b0 = b0;
	filter->b1 = b1;
	filter->b2 = b2;
	filter->a1 = a1;
	filter->a2 = a2;
	filter->leftZ1 = z1;
	filter->leftZ2 = z2;
	filter->rightZ1 = z1;
	filter->rightZ2 = z2;
}

static float xiaLimiterGainStep(float level, float *envelope, float *gain) {
	const float threshold = 0.99f;
	const float attackCoeff = 0.959f;
	const float releaseCoeff = 0.9999f;
	const float gainSlew = 0.04f;
	if (level > *envelope) {
		*envelope = attackCoeff * *envelope + (1.0f - attackCoeff) * level;
	} else {
		*envelope = releaseCoeff * *envelope + (1.0f - releaseCoeff) * level;
	}
	float target = *envelope > threshold ? threshold / *envelope : 1.0f;
	*gain += (target - *gain) * gainSlew;
	return *gain;
}

static float xiaClampUnitFloat(float value) {
	if (!isfinite(value)) {
		return 0;
	}
	if (value < 0) {
		return 0;
	}
	if (value > 1) {
		return 1;
	}
	return value;
}

static inline float xiaMonoSampleAt(
	int frame,
	const float *left,
	const float *right,
	const float *interleaved,
	UInt32 channels
) {
	if (interleaved != NULL) {
		UInt32 offset = (UInt32)frame * channels;
		float l = interleaved[offset];
		float r = channels >= 2 ? interleaved[offset + 1] : l;
		return (l + r) * 0.5f;
	}
	float l = left[frame];
	float r = right != NULL ? right[frame] : l;
	return (l + r) * 0.5f;
}

static void xiaUpdateAnalyzerCoefficients(double sampleRate) {
	if (sampleRate <= 0 || fabs(xiaAnalyzerSampleRate - sampleRate) < 0.5) {
		return;
	}
	xiaAnalyzerSampleRate = sampleRate;
	const double minFrequency = 45.0;
	double maxFrequency = fmin(16000.0, sampleRate * 0.45);
	if (maxFrequency <= minFrequency) {
		maxFrequency = minFrequency * 2.0;
	}
	for (int band = 0; band < xiaVisualizerBandCount; band++) {
		double ratio = (double)band / (double)(xiaVisualizerBandCount - 1);
		double frequency = minFrequency * pow(maxFrequency / minFrequency, ratio);
		double normalized = 2.0 * M_PI * frequency / sampleRate;
		xiaAnalyzerCoefficients[band] = 2.0 * cos(normalized);
	}
}

static void xiaClearVisualizerFrame(int running) {
	if (pthread_mutex_lock(&xiaVisualizerLock) != 0) {
		return;
	}
	memset(xiaVisualizerFrame.bands, 0, sizeof(xiaVisualizerFrame.bands));
	memset(xiaVisualizerFrame.waveform, 0, sizeof(xiaVisualizerFrame.waveform));
	xiaVisualizerFrame.level = 0;
	xiaVisualizerFrame.analysisTimeSeconds = 0;
	xiaVisualizerFrame.frameHostTimeSeconds = 0;
	xiaVisualizerFrame.running = running;
	xiaVisualizerFrame.sequence++;
	pthread_mutex_unlock(&xiaVisualizerLock);
}

static void xiaCommitVisualizerFrame(float *bands, float *waveform, float level, double durationSeconds, double frameHostTimeSeconds) {
	if (bands == NULL || waveform == NULL) {
		return;
	}
	if (pthread_mutex_trylock(&xiaVisualizerLock) != 0) {
		return;
	}
	for (int band = 0; band < xiaVisualizerBandCount; band++) {
		float current = xiaVisualizerFrame.bands[band];
		float target = xiaClampUnitFloat(bands[band]);
		float alpha = target > current ? 0.42f : 0.16f;
		xiaVisualizerFrame.bands[band] = current + (target - current) * alpha;
	}
	for (int index = 0; index < xiaVisualizerWaveformCount; index++) {
		float current = xiaVisualizerFrame.waveform[index];
		float target = waveform[index];
		if (!isfinite(target)) {
			target = 0;
		}
		if (target < -1) {
			target = -1;
		} else if (target > 1) {
			target = 1;
		}
		xiaVisualizerFrame.waveform[index] = current + (target - current) * 0.5f;
	}
	float currentLevel = xiaVisualizerFrame.level;
	float targetLevel = xiaClampUnitFloat(level);
	float levelAlpha = targetLevel > currentLevel ? 0.5f : 0.12f;
	xiaVisualizerFrame.level = currentLevel + (targetLevel - currentLevel) * levelAlpha;
	if (durationSeconds > 0 && isfinite(durationSeconds)) {
		xiaVisualizerFrame.analysisTimeSeconds += durationSeconds;
	}
	xiaVisualizerFrame.frameHostTimeSeconds = frameHostTimeSeconds > 0 ? frameHostTimeSeconds : xiaCurrentHostTimeSeconds();
	xiaVisualizerFrame.running = xiaEngine.running;
	xiaVisualizerFrame.sequence++;
	pthread_mutex_unlock(&xiaVisualizerLock);
}

static void xiaAnalyzeSamples(
	const float *left,
	const float *right,
	const float *interleaved,
	UInt32 channels,
	int frameCount,
	const AudioTimeStamp *outputTime
) {
	if (frameCount <= 8 || (left == NULL && interleaved == NULL) || (interleaved != NULL && channels == 0)) {
		return;
	}
	float bands[32];
	float waveform[64];
	memset(bands, 0, sizeof(bands));
	memset(waveform, 0, sizeof(waveform));

	double sumSquares = 0;
	for (int index = 0; index < xiaVisualizerWaveformCount; index++) {
		int start = (int)((int64_t)index * frameCount / xiaVisualizerWaveformCount);
		int end = (int)((int64_t)(index + 1) * frameCount / xiaVisualizerWaveformCount);
		if (end <= start) {
			end = start + 1;
		}
		double sum = 0;
		int count = 0;
		for (int frame = start; frame < end && frame < frameCount; frame++) {
			float sample = xiaMonoSampleAt(frame, left, right, interleaved, channels);
			sum += sample;
			count++;
		}
		waveform[index] = count > 0 ? (float)(sum / count) : 0;
	}

	for (int frame = 0; frame < frameCount; frame++) {
		float sample = xiaMonoSampleAt(frame, left, right, interleaved, channels);
		sumSquares += (double)sample * (double)sample;
	}

	for (int band = 0; band < xiaVisualizerBandCount; band++) {
		double q1 = 0;
		double q2 = 0;
		double coefficient = xiaAnalyzerCoefficients[band];
		for (int frame = 0; frame < frameCount; frame++) {
			float sample = xiaMonoSampleAt(frame, left, right, interleaved, channels);
			double q0 = coefficient * q1 - q2 + sample;
			q2 = q1;
			q1 = q0;
		}
		double power = q1 * q1 + q2 * q2 - coefficient * q1 * q2;
		if (power < 0 || !isfinite(power)) {
			power = 0;
		}
		double magnitude = sqrt(power) / (double)frameCount;
		double db = 20.0 * log10(magnitude * 3.0 + 1.0e-8);
		double normalized = (db + 58.0) / 58.0;
		bands[band] = xiaClampUnitFloat((float)normalized);
	}

	double rms = sqrt(sumSquares / (double)frameCount);
	double levelDB = 20.0 * log10(rms * 2.0 + 1.0e-8);
	float level = xiaClampUnitFloat((float)((levelDB + 54.0) / 54.0));
	double sampleRate = xiaAnalyzerSampleRate > 0 ? xiaAnalyzerSampleRate : xiaEngine.sampleRate;
	double durationSeconds = sampleRate > 0 ? (double)frameCount / sampleRate : 0;
	xiaCommitVisualizerFrame(bands, waveform, level, durationSeconds, xiaAudioFrameHostTimeSeconds(outputTime, durationSeconds));
}

static OSStatus xiaEqualizerIOProc(
	AudioObjectID inDevice,
	const AudioTimeStamp *inNow,
	const AudioBufferList *inInputData,
	const AudioTimeStamp *inInputTime,
	AudioBufferList *outOutputData,
	const AudioTimeStamp *inOutputTime,
	void *inClientData
) {
	(void)inDevice;
	(void)inNow;
	(void)inInputTime;
	(void)inOutputTime;
	(void)inClientData;

	if (inInputData == NULL || outOutputData == NULL || outOutputData->mNumberBuffers == 0) {
		return noErr;
	}
	UInt32 outBuffers = outOutputData->mNumberBuffers;
	UInt32 inBuffers = inInputData->mNumberBuffers;
	UInt32 bufferCount = inBuffers < outBuffers ? inBuffers : outBuffers;
	for (UInt32 bufferIndex = 0; bufferIndex < bufferCount; bufferIndex++) {
		AudioBuffer inBuffer = inInputData->mBuffers[bufferIndex];
		AudioBuffer *outBuffer = &outOutputData->mBuffers[bufferIndex];
		if (inBuffer.mData == NULL || outBuffer->mData == NULL) {
			continue;
		}
		UInt32 bytes = inBuffer.mDataByteSize < outBuffer->mDataByteSize ? inBuffer.mDataByteSize : outBuffer->mDataByteSize;
		memcpy(outBuffer->mData, inBuffer.mData, bytes);
		if (outBuffer->mDataByteSize > bytes) {
			memset((char *)outBuffer->mData + bytes, 0, outBuffer->mDataByteSize - bytes);
		}
		if (!xiaEngine.hasObservedAudio) {
			UInt32 sampleCount = bytes / (UInt32)sizeof(float);
			float *samples = (float *)inBuffer.mData;
			for (UInt32 sampleIndex = 0; sampleIndex < sampleCount; sampleIndex++) {
				if (samples[sampleIndex] != 0) {
					xiaEngine.hasObservedAudio = 1;
					break;
				}
			}
		}
	}
	for (UInt32 bufferIndex = bufferCount; bufferIndex < outBuffers; bufferIndex++) {
		AudioBuffer *outBuffer = &outOutputData->mBuffers[bufferIndex];
		if (outBuffer->mData != NULL && outBuffer->mDataByteSize > 0) {
			memset(outBuffer->mData, 0, outBuffer->mDataByteSize);
		}
	}

	float gain = xiaEngine.preampLinear;
	float mix = xiaEngine.wetMix;
	float target = xiaEngine.wetMixTarget;
	const float crossfadeAlpha = 0.002f;

	if (inBuffers >= 2 && outBuffers >= 2) {
		AudioBuffer *outLeftBuffer = &outOutputData->mBuffers[0];
		AudioBuffer *outRightBuffer = &outOutputData->mBuffers[1];
		const AudioBuffer *inLeftBuffer = &inInputData->mBuffers[0];
		const AudioBuffer *inRightBuffer = &inInputData->mBuffers[1];
		if (outLeftBuffer->mData != NULL && outRightBuffer->mData != NULL &&
			inLeftBuffer->mData != NULL && inRightBuffer->mData != NULL &&
			outLeftBuffer->mNumberChannels == 1 && outRightBuffer->mNumberChannels == 1) {
			UInt32 leftBytes = outLeftBuffer->mDataByteSize < inLeftBuffer->mDataByteSize ?
				outLeftBuffer->mDataByteSize : inLeftBuffer->mDataByteSize;
			UInt32 rightBytes = outRightBuffer->mDataByteSize < inRightBuffer->mDataByteSize ?
				outRightBuffer->mDataByteSize : inRightBuffer->mDataByteSize;
			UInt32 leftFrames = leftBytes / (UInt32)sizeof(float);
			UInt32 rightFrames = rightBytes / (UInt32)sizeof(float);
			UInt32 frames = leftFrames < rightFrames ? leftFrames : rightFrames;
			if (frames > 0) {
				float *left = (float *)outLeftBuffer->mData;
				float *right = (float *)outRightBuffer->mData;
				float *dryLeft = (float *)inLeftBuffer->mData;
				float *dryRight = (float *)inRightBuffer->mData;
				xiaAnalyzeSamples(dryLeft, dryRight, NULL, 0, (int)frames, inOutputTime);
				for (int filterIndex = 0; filterIndex < xiaBandCount; filterIndex++) {
					xiaProcessBiquadStereo(&xiaEngine.filters[filterIndex], left, right, (int)frames);
				}
				float env = xiaEngine.envStereo;
				float limiterGain = xiaEngine.limiterGainStereo;
				for (UInt32 index = 0; index < frames; index++) {
					mix += (target - mix) * crossfadeAlpha;
					float wetLeft = left[index] * gain;
					float wetRight = right[index] * gain;
					float linkedGain = xiaLimiterGainStep(fmaxf(fabsf(wetLeft), fabsf(wetRight)), &env, &limiterGain);
					wetLeft *= linkedGain;
					wetRight *= linkedGain;
					left[index] = dryLeft[index] * (1.0f - mix) + wetLeft * mix;
					right[index] = dryRight[index] * (1.0f - mix) + wetRight * mix;
				}
				xiaEngine.wetMix = mix;
				xiaEngine.envStereo = env;
				xiaEngine.limiterGainStereo = limiterGain;
				return noErr;
			}
		}
	}

	if (bufferCount == 0) {
		xiaEngine.wetMix = mix;
		return noErr;
	}
	AudioBuffer *outBuffer = &outOutputData->mBuffers[0];
	const AudioBuffer *inBuffer = &inInputData->mBuffers[0];
	if (outBuffer->mData == NULL || inBuffer->mData == NULL || outBuffer->mNumberChannels == 0) {
		xiaEngine.wetMix = mix;
		return noErr;
	}
	UInt32 channels = outBuffer->mNumberChannels;
	UInt32 bytes = outBuffer->mDataByteSize < inBuffer->mDataByteSize ?
		outBuffer->mDataByteSize : inBuffer->mDataByteSize;
	UInt32 frames = bytes / ((UInt32)sizeof(float) * channels);
	if (frames == 0) {
		xiaEngine.wetMix = mix;
		return noErr;
	}
	float *samples = (float *)outBuffer->mData;
	const float *drySamples = (const float *)inBuffer->mData;
	if (channels >= 2) {
		xiaAnalyzeSamples(NULL, NULL, drySamples, channels, (int)frames, inOutputTime);
		for (int filterIndex = 0; filterIndex < xiaBandCount; filterIndex++) {
			xiaProcessBiquadInterleavedStereo(&xiaEngine.filters[filterIndex], samples, (int)frames, channels);
		}
		float env = xiaEngine.envStereo;
		float limiterGain = xiaEngine.limiterGainStereo;
		for (UInt32 frame = 0; frame < frames; frame++) {
			mix += (target - mix) * crossfadeAlpha;
			UInt32 offset = frame * channels;
			float wetLeft = samples[offset] * gain;
			float wetRight = samples[offset + 1] * gain;
			float linkedGain = xiaLimiterGainStep(fmaxf(fabsf(wetLeft), fabsf(wetRight)), &env, &limiterGain);
			wetLeft *= linkedGain;
			wetRight *= linkedGain;
			samples[offset] = drySamples[offset] * (1.0f - mix) + wetLeft * mix;
			samples[offset + 1] = drySamples[offset + 1] * (1.0f - mix) + wetRight * mix;
		}
		xiaEngine.wetMix = mix;
		xiaEngine.envStereo = env;
		xiaEngine.limiterGainStereo = limiterGain;
		return noErr;
	}

	xiaAnalyzeSamples(NULL, NULL, drySamples, channels, (int)frames, inOutputTime);
	for (int filterIndex = 0; filterIndex < xiaBandCount; filterIndex++) {
		xiaProcessBiquadMono(&xiaEngine.filters[filterIndex], samples, (int)frames);
	}
	float env = xiaEngine.envMono;
	float limiterGain = xiaEngine.limiterGainMono;
	for (UInt32 frame = 0; frame < frames; frame++) {
		mix += (target - mix) * crossfadeAlpha;
		float wet = samples[frame] * gain;
		float linkedGain = xiaLimiterGainStep(fabsf(wet), &env, &limiterGain);
		wet *= linkedGain;
		samples[frame] = drySamples[frame] * (1.0f - mix) + wet * mix;
	}
	xiaEngine.wetMix = mix;
	xiaEngine.envMono = env;
	xiaEngine.limiterGainMono = limiterGain;
	return noErr;
}

static NSString *xiaStringProperty(AudioObjectID objectID, AudioObjectPropertySelector selector) {
	AudioObjectPropertyAddress address = {
		.mSelector = selector,
		.mScope = kAudioObjectPropertyScopeGlobal,
		.mElement = kAudioObjectPropertyElementMain,
	};
	CFStringRef value = NULL;
	UInt32 size = (UInt32)sizeof(CFStringRef);
	OSStatus status = AudioObjectGetPropertyData(objectID, &address, 0, NULL, &size, &value);
	if (status != noErr || value == NULL) {
		return nil;
	}
	NSString *result = [(NSString *)value copy];
	CFRelease(value);
	return [result autorelease];
}

static NSArray<NSNumber *> *xiaAllAudioProcessObjects(void) {
	AudioObjectPropertyAddress address = {
		.mSelector = kAudioHardwarePropertyProcessObjectList,
		.mScope = kAudioObjectPropertyScopeGlobal,
		.mElement = kAudioObjectPropertyElementMain,
	};
	UInt32 size = 0;
	OSStatus sizeStatus = AudioObjectGetPropertyDataSize(kAudioObjectSystemObject, &address, 0, NULL, &size);
	if (sizeStatus != noErr || size == 0) {
		return @[];
	}
	UInt32 count = size / (UInt32)sizeof(AudioObjectID);
	AudioObjectID *objects = calloc(count, sizeof(AudioObjectID));
	if (objects == NULL) {
		return @[];
	}
	OSStatus status = AudioObjectGetPropertyData(kAudioObjectSystemObject, &address, 0, NULL, &size, objects);
	NSMutableArray<NSNumber *> *result = [NSMutableArray array];
	if (status == noErr) {
		for (UInt32 index = 0; index < count; index++) {
			[result addObject:@(objects[index])];
		}
	}
	free(objects);
	return result;
}

static pid_t xiaProcessPID(AudioObjectID objectID) {
	AudioObjectPropertyAddress address = {
		.mSelector = kAudioProcessPropertyPID,
		.mScope = kAudioObjectPropertyScopeGlobal,
		.mElement = kAudioObjectPropertyElementMain,
	};
	pid_t pid = -1;
	UInt32 size = (UInt32)sizeof(pid_t);
	OSStatus status = AudioObjectGetPropertyData(objectID, &address, 0, NULL, &size, &pid);
	return status == noErr ? pid : -1;
}

static pid_t xiaParentPID(pid_t pid) {
	struct proc_bsdinfo info;
	memset(&info, 0, sizeof(info));
	int result = proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, &info, sizeof(info));
	return result == sizeof(info) ? (pid_t)info.pbi_ppid : -1;
}

#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
static NSString *xiaProcessName(pid_t pid) {
	ProcessSerialNumber psn;
	memset(&psn, 0, sizeof(psn));
	if (GetProcessForPID(pid, &psn) != noErr) {
		return nil;
	}
	CFStringRef name = NULL;
	if (CopyProcessName(&psn, &name) != noErr || name == NULL) {
		return nil;
	}
	NSString *result = [(NSString *)name copy];
	CFRelease(name);
	return [result autorelease];
}

static NSString *xiaLauncherProcessName(pid_t pid) {
	ProcessSerialNumber psn;
	memset(&psn, 0, sizeof(psn));
	if (GetProcessForPID(pid, &psn) != noErr) {
		return nil;
	}
	ProcessInfoRec info;
	memset(&info, 0, sizeof(info));
	info.processInfoLength = (UInt32)sizeof(info);
	if (GetProcessInformation(&psn, &info) != noErr) {
		return nil;
	}
	if (info.processLauncher.highLongOfPSN == 0 && info.processLauncher.lowLongOfPSN == 0) {
		return nil;
	}
	CFStringRef name = NULL;
	ProcessSerialNumber launcher = info.processLauncher;
	if (CopyProcessName(&launcher, &name) != noErr || name == NULL) {
		return nil;
	}
	NSString *result = [(NSString *)name copy];
	CFRelease(name);
	return [result autorelease];
}
#pragma clang diagnostic pop

static NSSet<NSNumber *> *xiaChildPIDs(pid_t parentPID) {
	int capacity = 8;
	while (capacity <= 4096) {
		pid_t *children = calloc((size_t)capacity, sizeof(pid_t));
		if (children == NULL) {
			return [NSSet set];
		}
		int byteCount = proc_listchildpids(parentPID, children, capacity * (int)sizeof(pid_t));
		if (byteCount < 0) {
			free(children);
			return [NSSet set];
		}
		int childCount = byteCount / (int)sizeof(pid_t);
		if (childCount < capacity) {
			NSMutableSet<NSNumber *> *result = [NSMutableSet set];
			for (int index = 0; index < childCount; index++) {
				if (children[index] > 0) {
					[result addObject:@(children[index])];
				}
			}
			free(children);
			return result;
		}
		free(children);
		capacity *= 2;
	}
	return [NSSet set];
}

static NSArray<NSString *> *xiaHostProcessNames(void) {
	NSMutableOrderedSet<NSString *> *names = [NSMutableOrderedSet orderedSet];
	NSString *processName = [[NSProcessInfo processInfo] processName];
	if (processName.length > 0) {
		[names addObject:processName];
	}
	NSString *displayName = [[NSBundle mainBundle] objectForInfoDictionaryKey:@"CFBundleDisplayName"];
	if (displayName.length > 0) {
		[names addObject:displayName];
	}
	NSString *bundleName = [[NSBundle mainBundle] objectForInfoDictionaryKey:(NSString *)kCFBundleNameKey];
	if (bundleName.length > 0) {
		[names addObject:bundleName];
	}
	[names addObject:@"XiaDown"];
	[names addObject:@"xiadown"];
	return names.array;
}

static BOOL xiaProcessNameLooksOwned(NSString *candidateName, NSArray<NSString *> *hostNames) {
	if (candidateName.length == 0) {
		return NO;
	}
	for (NSString *hostName in hostNames) {
		if (hostName.length == 0) {
			continue;
		}
		NSRange range = [candidateName rangeOfString:hostName options:(NSCaseInsensitiveSearch | NSAnchoredSearch)];
		if (range.location != NSNotFound) {
			return YES;
		}
	}
	return NO;
}

static BOOL xiaIsWebKitAudioProcessBundle(NSString *bundleID) {
	if (bundleID.length == 0) {
		return NO;
	}
	return [bundleID isEqualToString:@"com.apple.WebKit.WebContent"] ||
		[bundleID isEqualToString:@"com.apple.WebKit.GPU"];
}

static void xiaAddProcessID(NSMutableSet<NSNumber *> *processIDs, pid_t pid) {
	if (processIDs == nil || pid <= 0) {
		return;
	}
	[processIDs addObject:@(pid)];
}

static void xiaAddWKWebViewProcessID(WKWebView *webView, NSMutableSet<NSNumber *> *processIDs, NSString *selectorName) {
	if (webView == nil || processIDs == nil || selectorName.length == 0) {
		return;
	}
	SEL selector = NSSelectorFromString(selectorName);
	if (![webView respondsToSelector:selector]) {
		return;
	}
	pid_t (*sendPID)(id, SEL) = (pid_t (*)(id, SEL))objc_msgSend;
	xiaAddProcessID(processIDs, sendPID(webView, selector));
}

static void xiaCollectWKWebViewProcessIDs(NSView *view, NSMutableSet<NSNumber *> *processIDs) {
	if (view == nil || processIDs == nil) {
		return;
	}
	if ([view isKindOfClass:[WKWebView class]]) {
		WKWebView *webView = (WKWebView *)view;
		xiaAddWKWebViewProcessID(webView, processIDs, @"_webProcessIdentifier");
		xiaAddWKWebViewProcessID(webView, processIDs, @"_gpuProcessIdentifier");
		return;
	}
	for (NSView *subview in view.subviews) {
		xiaCollectWKWebViewProcessIDs(subview, processIDs);
	}
}

static NSSet<NSNumber *> *xiaOwnedAudioProcessIDs(NSSet<NSNumber *> *childPIDs) {
	NSMutableSet<NSNumber *> *processIDs = [NSMutableSet set];
	xiaAddProcessID(processIDs, getpid());
	[processIDs unionSet:childPIDs];
	if ([NSThread isMainThread]) {
		for (NSWindow *window in NSApp.windows) {
			xiaCollectWKWebViewProcessIDs(window.contentView, processIDs);
		}
	} else {
		dispatch_sync(dispatch_get_main_queue(), ^{
			for (NSWindow *window in NSApp.windows) {
				xiaCollectWKWebViewProcessIDs(window.contentView, processIDs);
			}
		});
	}
	return processIDs;
}

static NSArray<NSNumber *> *xiaAudioObjectsToTap(void) {
	NSArray<NSNumber *> *objects = xiaAllAudioProcessObjects();
	NSMutableArray<NSNumber *> *result = [NSMutableArray array];
	pid_t ourPID = getpid();
	NSSet<NSNumber *> *childPIDs = xiaChildPIDs(ourPID);
	NSSet<NSNumber *> *ownedPIDs = xiaOwnedAudioProcessIDs(childPIDs);
	NSArray<NSString *> *hostNames = xiaHostProcessNames();

	for (NSNumber *objectNumber in objects) {
		AudioObjectID objectID = (AudioObjectID)objectNumber.unsignedIntValue;
		NSString *bundleID = xiaStringProperty(objectID, kAudioProcessPropertyBundleID);
		if (!xiaIsWebKitAudioProcessBundle(bundleID)) {
			continue;
		}
		pid_t pid = xiaProcessPID(objectID);
		if (pid <= 0) {
			continue;
		}
		if ([ownedPIDs containsObject:@(pid)]) {
			[result addObject:objectNumber];
			continue;
		}
		pid_t parentPID = xiaParentPID(pid);
		if (parentPID == ourPID ||
			[childPIDs containsObject:@(pid)] ||
			xiaProcessNameLooksOwned(xiaProcessName(pid), hostNames) ||
			xiaProcessNameLooksOwned(xiaLauncherProcessName(pid), hostNames)) {
			[result addObject:objectNumber];
		}
	}
	return result;
}

static AudioObjectID xiaDefaultOutputDevice(void) {
	AudioObjectPropertyAddress address = {
		.mSelector = kAudioHardwarePropertyDefaultOutputDevice,
		.mScope = kAudioObjectPropertyScopeGlobal,
		.mElement = kAudioObjectPropertyElementMain,
	};
	AudioObjectID deviceID = kAudioObjectUnknown;
	UInt32 size = (UInt32)sizeof(AudioObjectID);
	OSStatus status = AudioObjectGetPropertyData(kAudioObjectSystemObject, &address, 0, NULL, &size, &deviceID);
	if (status != noErr || deviceID == kAudioObjectUnknown) {
		return kAudioObjectUnknown;
	}
	return deviceID;
}

static NSString *xiaDefaultOutputDeviceUID(void) {
	AudioObjectID deviceID = xiaDefaultOutputDevice();
	if (deviceID == kAudioObjectUnknown) {
		return nil;
	}
	return xiaStringProperty(deviceID, kAudioDevicePropertyDeviceUID);
}

static void xiaClearBoundOutputDeviceUIDLocked(void) {
	if (xiaEngine.outputDeviceUID == NULL) {
		return;
	}
	CFRelease(xiaEngine.outputDeviceUID);
	xiaEngine.outputDeviceUID = NULL;
}

static void xiaAdoptBoundOutputDeviceUIDLocked(NSString *outputUID) {
	xiaClearBoundOutputDeviceUIDLocked();
	if (outputUID.length == 0) {
		[outputUID release];
		return;
	}
	xiaEngine.outputDeviceUID = (CFStringRef)outputUID;
}

static void xiaDestroyOrphanedAggregates(void) {
	AudioObjectPropertyAddress address = {
		.mSelector = kAudioHardwarePropertyDevices,
		.mScope = kAudioObjectPropertyScopeGlobal,
		.mElement = kAudioObjectPropertyElementMain,
	};
	UInt32 size = 0;
	if (AudioObjectGetPropertyDataSize(kAudioObjectSystemObject, &address, 0, NULL, &size) != noErr || size == 0) {
		return;
	}
	UInt32 count = size / (UInt32)sizeof(AudioObjectID);
	AudioObjectID *devices = calloc(count, sizeof(AudioObjectID));
	if (devices == NULL) {
		return;
	}
	if (AudioObjectGetPropertyData(kAudioObjectSystemObject, &address, 0, NULL, &size, devices) == noErr) {
		NSString *prefix = [NSString stringWithUTF8String:xiaAggregateUIDPrefix];
		for (UInt32 index = 0; index < count; index++) {
			NSString *uid = xiaStringProperty(devices[index], kAudioDevicePropertyDeviceUID);
			if ([uid hasPrefix:prefix]) {
				AudioHardwareDestroyAggregateDevice(devices[index]);
			}
		}
	}
	free(devices);
}

static AudioObjectID xiaCreateAggregateDevice(AudioObjectID tapID, NSString **boundOutputUID) {
	NSString *outputUID = xiaDefaultOutputDeviceUID();
	NSString *tapUID = xiaStringProperty(tapID, kAudioTapPropertyUID);
	if (outputUID.length == 0 || tapUID.length == 0) {
		return kAudioObjectUnknown;
	}
	NSString *aggregateUID = [NSString stringWithFormat:@"%s%@", xiaAggregateUIDPrefix, [[NSUUID UUID] UUIDString]];
	NSDictionary *description = @{
		xiaAudioKey(kAudioAggregateDeviceUIDKey): aggregateUID,
		xiaAudioKey(kAudioAggregateDeviceNameKey): [NSString stringWithUTF8String:xiaAggregateName],
		xiaAudioKey(kAudioAggregateDeviceIsPrivateKey): @YES,
		xiaAudioKey(kAudioAggregateDeviceIsStackedKey): @NO,
		xiaAudioKey(kAudioAggregateDeviceMainSubDeviceKey): outputUID,
		xiaAudioKey(kAudioAggregateDeviceTapAutoStartKey): @YES,
		xiaAudioKey(kAudioAggregateDeviceSubDeviceListKey): @[
			@{
				xiaAudioKey(kAudioSubDeviceUIDKey): outputUID,
			},
		],
		xiaAudioKey(kAudioAggregateDeviceTapListKey): @[
			@{
				xiaAudioKey(kAudioSubTapUIDKey): tapUID,
				xiaAudioKey(kAudioSubTapDriftCompensationKey): @YES,
			},
		],
	};
	AudioObjectID aggregateID = kAudioObjectUnknown;
	OSStatus status = AudioHardwareCreateAggregateDevice((CFDictionaryRef)description, &aggregateID);
	if (status != noErr) {
		return kAudioObjectUnknown;
	}
	if (boundOutputUID != NULL) {
		*boundOutputUID = [outputUID copy];
	}
	return aggregateID;
}

static int xiaReadAggregateSampleRate(AudioObjectID aggregateID, Float64 *sampleRate) {
	AudioObjectPropertyAddress address = {
		.mSelector = kAudioDevicePropertyNominalSampleRate,
		.mScope = kAudioObjectPropertyScopeGlobal,
		.mElement = kAudioObjectPropertyElementMain,
	};
	UInt32 size = (UInt32)sizeof(Float64);
	Float64 value = 0;
	OSStatus status = AudioObjectGetPropertyData(aggregateID, &address, 0, NULL, &size, &value);
	if (status != noErr || value <= 0) {
		return 0;
	}
	*sampleRate = value;
	return 1;
}

static void xiaDestroyProcessTap(AudioObjectID tapID) {
	if (tapID == kAudioObjectUnknown) {
		return;
	}
	if (@available(macOS 14.2, *)) {
		AudioHardwareDestroyProcessTap(tapID);
	}
}

static BOOL xiaStatusLooksPermissionDenied(OSStatus status) {
	return status == kAudioDevicePermissionsError;
}

static void xiaStoreEqualizerSettingsLocked(int enabled, double preampDB, const double *gains, int gainCount) {
	xiaEngine.settingsEnabled = enabled ? 1 : 0;
	if (!isfinite(preampDB)) {
		preampDB = 0;
	}
	if (preampDB < -12) {
		preampDB = -12;
	} else if (preampDB > 12) {
		preampDB = 12;
	}
	xiaEngine.settingsPreampDB = preampDB;
	for (int index = 0; index < xiaBandCount; index++) {
		double value = (gains != NULL && index < gainCount) ? gains[index] : 0;
		if (!isfinite(value)) {
			value = 0;
		}
		if (value < -12) {
			value = -12;
		} else if (value > 12) {
			value = 12;
		}
		xiaEngine.settingsGainsDB[index] = value;
	}
}

static void xiaApplyStoredEqualizerSettingsLocked(void) {
	double peakBandGain = 0;
	for (int index = 0; index < xiaBandCount; index++) {
		double value = xiaEngine.settingsGainsDB[index];
		if (index == 0 || value > peakBandGain) {
			peakBandGain = value;
		}
	}
	double autoTrimDB = -fmax(0, xiaEngine.settingsPreampDB + peakBandGain) * 0.2;
	xiaEngine.preampLinear = (float)pow(10.0, (xiaEngine.settingsPreampDB + autoTrimDB) / 20.0);
	xiaEngine.wetMixTarget = xiaEngine.settingsEnabled ? 1.0f : 0.0f;
	double sampleRate = xiaEngine.sampleRate > 0 ? xiaEngine.sampleRate : 48000;
	for (int index = 0; index < xiaBandCount; index++) {
		if (index == 0) {
			xiaSetShelf(&xiaEngine.filters[index], 0, xiaBandFrequencies[index], xiaBandQ[index], xiaEngine.settingsGainsDB[index], sampleRate);
		} else if (index == xiaBandCount - 1) {
			xiaSetShelf(&xiaEngine.filters[index], 1, xiaBandFrequencies[index], xiaBandQ[index], xiaEngine.settingsGainsDB[index], sampleRate);
		} else {
			xiaSetPeakingEQ(&xiaEngine.filters[index], xiaBandFrequencies[index], xiaBandQ[index], xiaEngine.settingsGainsDB[index], sampleRate);
		}
	}
}

static void xiaApplyEqualizerSettingsLocked(int enabled, double preampDB, const double *gains, int gainCount) {
	xiaStoreEqualizerSettingsLocked(enabled, preampDB, gains, gainCount);
	xiaApplyStoredEqualizerSettingsLocked();
}

static void xiaScheduleOutputRouteRestart(void) {
	if (!__sync_bool_compare_and_swap(&xiaOutputRouteRestartScheduled, 0, 1)) {
		return;
	}
	dispatch_async(xiaOutputRouteQueue(), ^{
		xiaRestartForDefaultOutputChange();
	});
}

static AudioObjectPropertyAddress xiaDefaultOutputDevicePropertyAddress(void) {
	AudioObjectPropertyAddress address = {
		.mSelector = kAudioHardwarePropertyDefaultOutputDevice,
		.mScope = kAudioObjectPropertyScopeGlobal,
		.mElement = kAudioObjectPropertyElementMain,
	};
	return address;
}

static void xiaInstallDefaultOutputListenerLocked(void) {
	if (xiaEngine.defaultOutputListenerInstalled) {
		return;
	}
	AudioObjectPropertyAddress address = xiaDefaultOutputDevicePropertyAddress();
	OSStatus status = AudioObjectAddPropertyListener(
		kAudioObjectSystemObject,
		&address,
		xiaDefaultOutputDeviceChanged,
		NULL
	);
	if (status == noErr) {
		xiaEngine.defaultOutputListenerInstalled = 1;
	}
}

static void xiaRemoveDefaultOutputListenerLocked(void) {
	if (!xiaEngine.defaultOutputListenerInstalled) {
		return;
	}
	AudioObjectPropertyAddress address = xiaDefaultOutputDevicePropertyAddress();
	AudioObjectRemovePropertyListener(
		kAudioObjectSystemObject,
		&address,
		xiaDefaultOutputDeviceChanged,
		NULL
	);
	xiaEngine.defaultOutputListenerInstalled = 0;
	__sync_lock_test_and_set(&xiaOutputRouteRestartScheduled, 0);
}

static void xiaStopAudioPipelineLocked(int clearVisualizer) {
	if (xiaEngine.ioProcID != NULL && xiaEngine.aggregateID != kAudioObjectUnknown) {
		AudioDeviceStop(xiaEngine.aggregateID, xiaEngine.ioProcID);
		AudioDeviceDestroyIOProcID(xiaEngine.aggregateID, xiaEngine.ioProcID);
	}
	xiaEngine.ioProcID = NULL;
	if (xiaEngine.aggregateID != kAudioObjectUnknown) {
		AudioHardwareDestroyAggregateDevice(xiaEngine.aggregateID);
	}
	xiaEngine.aggregateID = kAudioObjectUnknown;
	if (xiaEngine.tapID != kAudioObjectUnknown) {
		xiaDestroyProcessTap(xiaEngine.tapID);
	}
	xiaEngine.tapID = kAudioObjectUnknown;
	xiaClearBoundOutputDeviceUIDLocked();
	xiaEngine.running = 0;
	xiaEngine.hasObservedAudio = 0;
	if (clearVisualizer) {
		xiaClearVisualizerFrame(0);
	}
}

static int xiaStartAudioPipelineLocked(int *detailStatus) {
	if (@available(macOS 14.2, *)) {
		xiaDestroyOrphanedAggregates();
		NSArray<NSNumber *> *objects = xiaAudioObjectsToTap();
		if (objects.count == 0) {
			return XiaEQStartNoAudioSource;
		}

		CATapDescription *description = [[CATapDescription alloc] initStereoMixdownOfProcesses:objects];
		description.muteBehavior = CATapMutedWhenTapped;
		description.privateTap = YES;
		description.exclusive = NO;
		description.name = [NSString stringWithUTF8String:xiaTapName];

		AudioObjectID tapID = kAudioObjectUnknown;
		OSStatus tapStatus = AudioHardwareCreateProcessTap(description, &tapID);
		[description release];
		if (tapStatus != noErr) {
			if (detailStatus != NULL) {
				*detailStatus = tapStatus;
			}
			xiaDestroyProcessTap(tapID);
			if (xiaStatusLooksPermissionDenied(tapStatus)) {
				return XiaEQStartPermissionDenied;
			}
			return XiaEQStartTapCreation;
		}
		xiaEngine.tapID = tapID;

		NSString *boundOutputUID = nil;
		AudioObjectID aggregateID = xiaCreateAggregateDevice(tapID, &boundOutputUID);
		if (aggregateID == kAudioObjectUnknown) {
			[boundOutputUID release];
			xiaStopAudioPipelineLocked(1);
			return XiaEQStartAggregateCreation;
		}
		xiaEngine.aggregateID = aggregateID;

		Float64 sampleRate = 0;
		if (!xiaReadAggregateSampleRate(aggregateID, &sampleRate)) {
			[boundOutputUID release];
			xiaStopAudioPipelineLocked(1);
			return XiaEQStartInvalidTapFormat;
		}
		xiaEngine.sampleRate = sampleRate;
		xiaUpdateAnalyzerCoefficients(sampleRate);
		xiaApplyStoredEqualizerSettingsLocked();
		xiaResetDSPState();

		AudioDeviceIOProcID ioProcID = NULL;
		OSStatus createStatus = AudioDeviceCreateIOProcID(aggregateID, xiaEqualizerIOProc, NULL, &ioProcID);
		if (createStatus != noErr || ioProcID == NULL) {
			if (detailStatus != NULL) {
				*detailStatus = createStatus;
			}
			if (ioProcID != NULL) {
				AudioDeviceDestroyIOProcID(aggregateID, ioProcID);
			}
			[boundOutputUID release];
			xiaStopAudioPipelineLocked(1);
			return XiaEQStartIOProcInstall;
		}
		xiaEngine.ioProcID = ioProcID;

		OSStatus startStatus = AudioDeviceStart(aggregateID, ioProcID);
		if (startStatus != noErr) {
			if (detailStatus != NULL) {
				*detailStatus = startStatus;
			}
			[boundOutputUID release];
			xiaStopAudioPipelineLocked(1);
			if (xiaStatusLooksPermissionDenied(startStatus)) {
				return XiaEQStartPermissionDenied;
			}
			return XiaEQStartDeviceStart;
		}

		xiaEngine.running = 1;
		xiaAdoptBoundOutputDeviceUIDLocked(boundOutputUID);
		xiaClearVisualizerFrame(1);
		xiaInstallDefaultOutputListenerLocked();
		NSString *currentUID = xiaDefaultOutputDeviceUID();
		if (
			currentUID.length > 0 &&
			xiaEngine.outputDeviceUID != NULL &&
			CFStringCompare(xiaEngine.outputDeviceUID, (CFStringRef)currentUID, 0) != kCFCompareEqualTo
		) {
			xiaScheduleOutputRouteRestart();
		}
		return XiaEQStartSuccess;
	}
	return XiaEQStartUnsupported;
}

static OSStatus xiaDefaultOutputDeviceChanged(
	AudioObjectID inObjectID,
	UInt32 inNumberAddresses,
	const AudioObjectPropertyAddress *inAddresses,
	void *inClientData
) {
	(void)inObjectID;
	(void)inNumberAddresses;
	(void)inAddresses;
	(void)inClientData;
	xiaScheduleOutputRouteRestart();
	return noErr;
}

static void xiaRestartForDefaultOutputChange(void) {
	if (@available(macOS 14.2, *)) {
		@autoreleasepool {
			pthread_mutex_lock(&xiaEngineLock);
			__sync_lock_test_and_set(&xiaOutputRouteRestartScheduled, 0);
			xiaEnsureEngineInitialized();
			if (!xiaEngine.running) {
				pthread_mutex_unlock(&xiaEngineLock);
				return;
			}
			NSString *currentUID = xiaDefaultOutputDeviceUID();
			if (currentUID.length == 0) {
				pthread_mutex_unlock(&xiaEngineLock);
				return;
			}
			if (
				xiaEngine.outputDeviceUID != NULL &&
				CFStringCompare(xiaEngine.outputDeviceUID, (CFStringRef)currentUID, 0) == kCFCompareEqualTo
			) {
				pthread_mutex_unlock(&xiaEngineLock);
				return;
			}

			xiaStopAudioPipelineLocked(0);
			int status = xiaStartAudioPipelineLocked(NULL);
			if (status != XiaEQStartSuccess) {
				xiaRemoveDefaultOutputListenerLocked();
				xiaClearVisualizerFrame(0);
			}
			pthread_mutex_unlock(&xiaEngineLock);
		}
		return;
	}
	__sync_lock_test_and_set(&xiaOutputRouteRestartScheduled, 0);
}

int xia_equalizer_supported(void) {
	if (@available(macOS 14.2, *)) {
		return 1;
	}
	return 0;
}

int xia_equalizer_has_capture_permission(void) {
	if (@available(macOS 10.15, *)) {
		return CGPreflightScreenCaptureAccess() ? 1 : 0;
	}
	return 1;
}

int xia_equalizer_request_capture_permission(void) {
	if (@available(macOS 10.15, *)) {
		return CGRequestScreenCaptureAccess() ? 1 : 0;
	}
	return 1;
}

int xia_equalizer_is_running(void) {
	pthread_mutex_lock(&xiaEngineLock);
	xiaEnsureEngineInitialized();
	int running = xiaEngine.running;
	pthread_mutex_unlock(&xiaEngineLock);
	return running;
}

int xia_equalizer_has_observed_audio(void) {
	xiaEnsureEngineInitialized();
	return xiaEngine.hasObservedAudio;
}

int xia_equalizer_visualizer_frame(
	float *bands,
	int bandCount,
	float *waveform,
	int waveformCount,
	double *level,
	unsigned long long *sequence,
	double *analysisTimeSeconds,
	double *frameTimeOffsetSeconds
) {
	if (bands != NULL && bandCount > 0) {
		memset(bands, 0, (size_t)bandCount * sizeof(float));
	}
	if (waveform != NULL && waveformCount > 0) {
		memset(waveform, 0, (size_t)waveformCount * sizeof(float));
	}
	if (level != NULL) {
		*level = 0;
	}
	if (sequence != NULL) {
		*sequence = 0;
	}
	if (analysisTimeSeconds != NULL) {
		*analysisTimeSeconds = 0;
	}
	if (frameTimeOffsetSeconds != NULL) {
		*frameTimeOffsetSeconds = 0;
	}
	if (pthread_mutex_lock(&xiaVisualizerLock) != 0) {
		return 0;
	}
	int running = xiaVisualizerFrame.running;
	int copyBands = bandCount < xiaVisualizerBandCount ? bandCount : xiaVisualizerBandCount;
	int copyWaveform = waveformCount < xiaVisualizerWaveformCount ? waveformCount : xiaVisualizerWaveformCount;
	if (bands != NULL && copyBands > 0) {
		memcpy(bands, xiaVisualizerFrame.bands, (size_t)copyBands * sizeof(float));
	}
	if (waveform != NULL && copyWaveform > 0) {
		memcpy(waveform, xiaVisualizerFrame.waveform, (size_t)copyWaveform * sizeof(float));
	}
	if (level != NULL) {
		*level = xiaVisualizerFrame.level;
	}
	if (sequence != NULL) {
		*sequence = (unsigned long long)xiaVisualizerFrame.sequence;
	}
	if (analysisTimeSeconds != NULL) {
		*analysisTimeSeconds = xiaVisualizerFrame.analysisTimeSeconds;
	}
	if (frameTimeOffsetSeconds != NULL && xiaVisualizerFrame.frameHostTimeSeconds > 0) {
		double offset = xiaCurrentHostTimeSeconds() - xiaVisualizerFrame.frameHostTimeSeconds;
		if (!isfinite(offset)) {
			offset = 0;
		}
		*frameTimeOffsetSeconds = offset;
	}
	pthread_mutex_unlock(&xiaVisualizerLock);
	return running;
}

void xia_equalizer_apply(int enabled, double preampDB, const double *gains, int gainCount) {
	pthread_mutex_lock(&xiaEngineLock);
	xiaEnsureEngineInitialized();
	xiaApplyEqualizerSettingsLocked(enabled, preampDB, gains, gainCount);
	pthread_mutex_unlock(&xiaEngineLock);
}

void xia_equalizer_stop(void) {
	pthread_mutex_lock(&xiaEngineLock);
	xiaEnsureEngineInitialized();
	xiaRemoveDefaultOutputListenerLocked();
	xiaStopAudioPipelineLocked(1);
	pthread_mutex_unlock(&xiaEngineLock);
}

int xia_equalizer_start(int enabled, double preampDB, const double *gains, int gainCount, int *detailStatus) {
	if (detailStatus != NULL) {
		*detailStatus = 0;
	}
	if (!xia_equalizer_supported()) {
		return XiaEQStartUnsupported;
	}
	if (!xia_equalizer_has_capture_permission()) {
		return XiaEQStartPermissionDenied;
	}

	if (@available(macOS 14.2, *)) {
		@autoreleasepool {
			pthread_mutex_lock(&xiaEngineLock);
			xiaEnsureEngineInitialized();
			xiaApplyEqualizerSettingsLocked(enabled, preampDB, gains, gainCount);
			if (xiaEngine.running) {
				pthread_mutex_unlock(&xiaEngineLock);
				return XiaEQStartSuccess;
			}
			int status = xiaStartAudioPipelineLocked(detailStatus);
			pthread_mutex_unlock(&xiaEngineLock);
			return status;
		}
	}
	return XiaEQStartUnsupported;
}
