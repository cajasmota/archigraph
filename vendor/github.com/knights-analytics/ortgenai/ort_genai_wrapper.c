#include "ort_genai_wrapper.h"

static GenAiApiTable g_api = {0}; // api table
static int g_initialized = 0;
static int g_engine_initialized = 0;

int SetGenAiApi(void* createModel,
				void* resultGetError,
				void* destroyResult,
				void* destroyModel,
				void* createTokenizer,
				void* destroyTokenizer,
				void* createTokenizerStream,
				void* destroyTokenizerStream,
				void* applyChatTemplate,
				void* destroyString,
				void* createSequences,
				void* destroySequences,
			void* tokenizerEncode,
			void* createGenerator,
			void* destroyGenerator,
			void* createGeneratorParams,
			void* destroyGeneratorParams,
			void* generatorParamsSetSearchNumber,
			void* generatorAppendTokenSequences,
			void* generatorSetInputs,
			void* generatorGenerateNextToken,
			void* generatorGetSequenceCount,
			void* generatorGetSequenceData,
			void* tokenizerStreamDecode,
			void* isDone,
			void* tokenizerGetEosTokenIds,
			// Config
			void* createConfig,
			void* configClearProviders,
			void* configAppendProvider,
			void* configSetProviderOption,
			void* createModelFromConfig,
			void* destroyConfig,
			// Multimodal
			void* loadImage,
			void* loadImages,
			void* loadImagesFromBuffers,
			void* destroyImages,
			void* createMultiModalProcessor,
			void* destroyMultiModalProcessor,
			void* processorProcessImages,
			void* destroyNamedTensors,
			void* createStringArray,
			void* destroyStringArray,
			void* stringArrayAddString,
			void* processorProcessImagesAndPrompts,
	// Guidance/constrained-generation (optional — may be NULL on older runtimes)
	void* generatorParamsSetGuidance) {
	if (g_initialized) return 0; // already initialized
	// Validate all required pointers (header comment: all must be non-null)
	if (!createModel || !resultGetError || !destroyResult || !destroyModel ||
		!createTokenizer || !destroyTokenizer || !createTokenizerStream || !destroyTokenizerStream ||
		!applyChatTemplate || !destroyString || !createSequences || !destroySequences || !tokenizerEncode ||
		!createGenerator || !destroyGenerator || !createGeneratorParams || !destroyGeneratorParams ||
		!generatorParamsSetSearchNumber || !generatorAppendTokenSequences || !generatorSetInputs || !generatorGenerateNextToken ||
		!generatorGetSequenceCount || !generatorGetSequenceData || !tokenizerStreamDecode || !isDone || !tokenizerGetEosTokenIds ||
		// Config
		!createConfig || !configClearProviders || !configAppendProvider || !configSetProviderOption || !createModelFromConfig || !destroyConfig ||
		// Multimodal
		!loadImage || !loadImages || !loadImagesFromBuffers || !destroyImages ||
		!createMultiModalProcessor || !destroyMultiModalProcessor || !processorProcessImages ||
		!destroyNamedTensors || !createStringArray || !destroyStringArray || !stringArrayAddString || !processorProcessImagesAndPrompts) {
		return 1;
	}
	g_api.CreateModel = (PFN_OgaCreateModel) createModel;
	g_api.ResultGetError = (PFN_OgaResultGetError) resultGetError;
	g_api.DestroyResult = (PFN_OgaDestroyResult) destroyResult;
	g_api.DestroyModel = (PFN_OgaDestroyModel) destroyModel;
	g_api.CreateTokenizer = (PFN_OgaCreateTokenizer) createTokenizer;
	g_api.DestroyTokenizer = (PFN_OgaDestroyTokenizer) destroyTokenizer;
	g_api.CreateTokenizerStream = (PFN_OgaCreateTokenizerStream) createTokenizerStream;
	g_api.DestroyTokenizerStream = (PFN_OgaDestroyTokenizerStream) destroyTokenizerStream;
	g_api.ApplyChatTemplate = (PFN_OgaTokenizerApplyChatTemplate) applyChatTemplate;
	g_api.DestroyString = (PFN_OgaDestroyString) destroyString;
	g_api.CreateSequences = (PFN_OgaCreateSequences) createSequences;
	g_api.DestroySequences = (PFN_OgaDestroySequences) destroySequences;
	g_api.TokenizerEncode = (PFN_OgaTokenizerEncode) tokenizerEncode;
	g_api.CreateGenerator = (PFN_OgaCreateGenerator) createGenerator;
	g_api.DestroyGenerator = (PFN_OgaDestroyGenerator) destroyGenerator;
	g_api.CreateGeneratorParams = (PFN_OgaCreateGeneratorParams) createGeneratorParams;
	g_api.DestroyGeneratorParams = (PFN_OgaDestroyGeneratorParams) destroyGeneratorParams;
	g_api.GeneratorParamsSetSearchNumber = (PFN_OgaGeneratorParamsSetSearchNumber) generatorParamsSetSearchNumber;
	g_api.GeneratorAppendTokenSequences = (PFN_OgaGeneratorAppendTokenSequences) generatorAppendTokenSequences;
	g_api.GeneratorSetInputs = (PFN_OgaGeneratorSetInputs) generatorSetInputs;
	g_api.GeneratorGenerateNextToken = (PFN_OgaGeneratorGenerateNextToken) generatorGenerateNextToken;
	g_api.GeneratorGetSequenceCount = (PFN_OgaGeneratorGetSequenceCount) generatorGetSequenceCount;
	g_api.GeneratorGetSequenceData = (PFN_OgaGeneratorGetSequenceData) generatorGetSequenceData;
	g_api.TokenizerStreamDecode = (PFN_OgaTokenizerStreamDecode) tokenizerStreamDecode;
	g_api.IsDone = (PFN_OgaGeneratorIsDone) isDone;
	g_api.TokenizerGetEosTokenIds = (PFN_OgaTokenizerGetEosTokenIds) tokenizerGetEosTokenIds;
	// Config
	g_api.CreateConfig = (PFN_OgaCreateConfig) createConfig;
	g_api.ConfigClearProviders = (PFN_OgaConfigClearProviders) configClearProviders;
	g_api.ConfigAppendProvider = (PFN_OgaConfigAppendProvider) configAppendProvider;
	g_api.ConfigSetProviderOption = (PFN_OgaConfigSetProviderOption) configSetProviderOption;
	g_api.CreateModelFromConfig = (PFN_OgaCreateModelFromConfig) createModelFromConfig;
	g_api.DestroyConfig = (PFN_OgaDestroyConfig) destroyConfig;
	// Multimodal
	g_api.LoadImage = (PFN_OgaLoadImage) loadImage;
	g_api.LoadImages = (PFN_OgaLoadImages) loadImages;
	g_api.LoadImagesFromBuffers = (PFN_OgaLoadImagesFromBuffers) loadImagesFromBuffers;
	g_api.DestroyImages = (PFN_OgaDestroyImages) destroyImages;
	g_api.CreateMultiModalProcessor = (PFN_OgaCreateMultiModalProcessor) createMultiModalProcessor;
	g_api.DestroyMultiModalProcessor = (PFN_OgaDestroyMultiModalProcessor) destroyMultiModalProcessor;
	g_api.ProcessorProcessImages = (PFN_OgaProcessorProcessImages) processorProcessImages;
	g_api.DestroyNamedTensors = (PFN_OgaDestroyNamedTensors) destroyNamedTensors;
	g_api.CreateStringArray = (PFN_OgaCreateStringArray) createStringArray;
	g_api.DestroyStringArray = (PFN_OgaDestroyStringArray) destroyStringArray;
	g_api.StringArrayAddString = (PFN_OgaStringArrayAddString) stringArrayAddString;
	g_api.ProcessorProcessImagesAndPrompts = (PFN_OgaProcessorProcessImagesAndPrompts) processorProcessImagesAndPrompts;
	if (generatorParamsSetGuidance) g_api.GeneratorParamsSetGuidance = (PFN_OgaGeneratorParamsSetGuidance) generatorParamsSetGuidance;
	g_initialized = 1;
	return 0;
}

int GenAiApiIsInitialized(void) { return g_initialized; }

OgaResult* CreateOgaModel(const char* config_path, OgaModel** out) {
	if (!g_initialized || !g_api.CreateModel) return NULL;
	return g_api.CreateModel(config_path, out);
}

OgaResult* CreateOgaTokenizer(const OgaModel* model, OgaTokenizer** out) {
	if (!g_initialized || !g_api.CreateTokenizer) return NULL;
	return g_api.CreateTokenizer(model, out);
}

OgaResult* CreateOgaTokenizerStream(const OgaTokenizer* tokenizer, OgaTokenizerStream** out) {
	if (!g_initialized || !g_api.CreateTokenizerStream) return NULL;
	return g_api.CreateTokenizerStream(tokenizer, out);
}

const char* GetOgaResultErrorString(const OgaResult* result) {
	if (!g_initialized || !g_api.ResultGetError) return "GenAI API not initialized";
	return g_api.ResultGetError(result);
}

void DestroyOgaResult(OgaResult* result) {
	if (!result) return;
	if (!g_initialized || !g_api.DestroyResult) return;
	g_api.DestroyResult(result);
}

void DestroyOgaModel(OgaModel* model) {
	if (!model) return;
	if (!g_initialized || !g_api.DestroyModel) return;
	g_api.DestroyModel(model);
}

void DestroyOgaTokenizer(OgaTokenizer* tokenizer) {
	if (!tokenizer) return;
	if (!g_initialized || !g_api.DestroyTokenizer) return;
	g_api.DestroyTokenizer(tokenizer);
}

void DestroyOgaTokenizerStream(OgaTokenizerStream* tokenizerStream) {
	if (!tokenizerStream) return;
	if (!g_initialized || !g_api.DestroyTokenizerStream) return;
	g_api.DestroyTokenizerStream(tokenizerStream);
}

void DestroyOgaString(const char* str) {
	if (!str) return;
	if (!g_initialized || !g_api.DestroyString) return;
	g_api.DestroyString(str);
}

OgaResult* ApplyOgaTokenizerChatTemplate(const OgaTokenizer* tokenizer, const char* input, const char* param1, const char* param2, bool flag, const char** output) {
	if (!g_initialized || !g_api.ApplyChatTemplate) return NULL;
	return g_api.ApplyChatTemplate(tokenizer, input, param1, param2, flag, output);
}

OgaResult* CreateOgaSequences(OgaSequences** out) {
	if (!g_initialized || !g_api.CreateSequences) return NULL;
	return g_api.CreateSequences(out);
}

void DestroyOgaSequences(OgaSequences* sequences) {
	if (!sequences) return;
	if (!g_initialized || !g_api.DestroySequences) return;
	g_api.DestroySequences(sequences);
}

OgaResult* TokenizerEncode(const OgaTokenizer* tokenizer, const char* str, OgaSequences* sequences) {
	if (!g_initialized || !g_api.TokenizerEncode) return NULL;
	return g_api.TokenizerEncode(tokenizer, str, sequences);
}

OgaResult* CreateOgaGenerator(const OgaModel* model, const OgaGeneratorParams* generatorParams, OgaGenerator** out) {
	if (!g_initialized || !g_api.CreateGenerator) return NULL;
	return g_api.CreateGenerator(model, generatorParams, out);
}

void DestroyOgaGenerator(OgaGenerator* generator) {
	if (!generator) return;
	if (!g_initialized || !g_api.DestroyGenerator) return;
	g_api.DestroyGenerator(generator);
}

OgaResult* CreateOgaGeneratorParams(const OgaModel* model,OgaGeneratorParams** out) {
	if (!g_initialized || !g_api.CreateGeneratorParams) return NULL;
	return g_api.CreateGeneratorParams(model, out);
}

void DestroyOgaGeneratorParams(OgaGeneratorParams* generatorParams) {
	if (!generatorParams) return;
	if (!g_initialized || !g_api.DestroyGeneratorParams) return;
	g_api.DestroyGeneratorParams(generatorParams);
}

OgaResult* GeneratorParamsSetSearchNumber(OgaGeneratorParams* generatorParams, const char* name, double searchNumber) {
    if (!g_initialized || !g_api.GeneratorParamsSetSearchNumber) return NULL;
    return g_api.GeneratorParamsSetSearchNumber(generatorParams, name, searchNumber);
}

OgaResult* GeneratorParamsSetGuidance(OgaGeneratorParams* params, const char* type, const char* data, bool enable_ff_tokens) {
	if (!g_initialized || !g_api.GeneratorParamsSetGuidance) return NULL;
	return g_api.GeneratorParamsSetGuidance(params, type, data, enable_ff_tokens);
}

OgaResult* GeneratorAppendTokenSequences(OgaGenerator* generator, OgaSequences* sequences) {
	if (!g_initialized || !g_api.GeneratorAppendTokenSequences) return NULL;
	return g_api.GeneratorAppendTokenSequences(generator, sequences);
}

OgaResult* GeneratorSetInputs(OgaGenerator* generator, const OgaNamedTensors* named_tensors) {
	if (!g_initialized || !g_api.GeneratorSetInputs) return NULL;
	return g_api.GeneratorSetInputs(generator, named_tensors);
}

OgaResult* GeneratorGenerateNextToken(OgaGenerator* generator) {
	if (!g_initialized || !g_api.GeneratorGenerateNextToken) return NULL;
	return g_api.GeneratorGenerateNextToken(generator);
}

size_t GeneratorGetSequenceCount(const OgaGenerator* generator, size_t sequence_index) {
	if (!g_initialized || !g_api.GeneratorGetSequenceCount) return 0;
	return g_api.GeneratorGetSequenceCount(generator, sequence_index);
}

const int32_t* GeneratorGetSequenceData(const OgaGenerator* generator, size_t sequence_index) {
	if (!g_initialized || !g_api.GeneratorGetSequenceData) return NULL;
	return g_api.GeneratorGetSequenceData(generator, sequence_index);
}

OgaResult* TokenizerStreamDecode(OgaTokenizerStream* tokenizerStream, int32_t token, const char** output) {
	if (!g_initialized || !g_api.TokenizerStreamDecode) return NULL;
	return g_api.TokenizerStreamDecode(tokenizerStream, token, output);
}

bool IsDone(const OgaGenerator* generator) {
	if (!g_initialized || !g_api.IsDone) return false;
	return g_api.IsDone(generator);
}

OgaResult* OgaTokenizerGetEosTokenIds(const OgaTokenizer* tokenizer, const int32_t** eos_token_ids, size_t* token_count) {
    if (!g_initialized || !g_api.TokenizerGetEosTokenIds) return NULL;
    return g_api.TokenizerGetEosTokenIds(tokenizer, eos_token_ids, token_count);
}

// Config thin wrappers
OgaResult* CreateOgaConfig(const char* config_path, OgaConfig** out) {
	if (!g_initialized || !g_api.CreateConfig) return NULL;
	return g_api.CreateConfig(config_path, out);
}

OgaResult* OgaConfigClearProviders(OgaConfig* config) {
	if (!g_initialized || !g_api.ConfigClearProviders) return NULL;
	return g_api.ConfigClearProviders(config);
}

OgaResult* OgaConfigAppendProvider(OgaConfig* config, const char* provider) {
	if (!g_initialized || !g_api.ConfigAppendProvider) return NULL;
	return g_api.ConfigAppendProvider(config, provider);
}

OgaResult* OgaConfigSetProviderOption(OgaConfig* config, const char* provider, const char* key, const char* value) {
	if (!g_initialized || !g_api.ConfigSetProviderOption) return NULL;
	return g_api.ConfigSetProviderOption(config, provider, key, value);
}

OgaResult* CreateOgaModelFromConfig(const OgaConfig* config, OgaModel** out) {
	if (!g_initialized || !g_api.CreateModelFromConfig) return NULL;
	return g_api.CreateModelFromConfig(config, out);
}

void DestroyOgaConfig(OgaConfig* config) {
	if (!config) return;
	if (!g_initialized || !g_api.DestroyConfig) return;
	g_api.DestroyConfig(config);
}

// Multimodal thin wrappers
OgaResult* LoadOgaImage(const char* image_path, OgaImages** out) {
	if (!g_initialized || !g_api.LoadImage) return NULL;
	return g_api.LoadImage(image_path, out);
}

OgaResult* LoadOgaImages(const OgaStringArray* image_paths, OgaImages** out) {
	if (!g_initialized || !g_api.LoadImages) return NULL;
	return g_api.LoadImages(image_paths, out);
}

OgaResult* LoadOgaImagesFromBuffers(const void** image_data, const size_t* image_data_sizes, size_t count, OgaImages** out) {
	if (!g_initialized || !g_api.LoadImagesFromBuffers) return NULL;
	return g_api.LoadImagesFromBuffers(image_data, image_data_sizes, count, out);
}

void DestroyOgaImages(OgaImages* images) {
	if (!images) return;
	if (!g_initialized || !g_api.DestroyImages) return;
	g_api.DestroyImages(images);
}

OgaResult* CreateOgaMultiModalProcessor(const OgaModel* model, OgaMultiModalProcessor** out) {
	if (!g_initialized || !g_api.CreateMultiModalProcessor) return NULL;
	return g_api.CreateMultiModalProcessor(model, out);
}

void DestroyOgaMultiModalProcessor(OgaMultiModalProcessor* processor) {
	if (!processor) return;
	if (!g_initialized || !g_api.DestroyMultiModalProcessor) return;
	g_api.DestroyMultiModalProcessor(processor);
}

OgaResult* ProcessOgaImages(const OgaMultiModalProcessor* processor, const char* prompt, const OgaImages* images, OgaNamedTensors** out) {
	if (!g_initialized || !g_api.ProcessorProcessImages) return NULL;
	return g_api.ProcessorProcessImages(processor, prompt, images, out);
}

void DestroyOgaNamedTensors(OgaNamedTensors* tensors) {
	if (!tensors) return;
	if (!g_initialized || !g_api.DestroyNamedTensors) return;
	g_api.DestroyNamedTensors(tensors);
}

OgaResult* CreateOgaStringArray(OgaStringArray** out) {
	if (!g_initialized || !g_api.CreateStringArray) return NULL;
	return g_api.CreateStringArray(out);
}

void DestroyOgaStringArray(OgaStringArray* string_array) {
	if (!string_array) return;
	if (!g_initialized || !g_api.DestroyStringArray) return;
	g_api.DestroyStringArray(string_array);
}

OgaResult* AddStringToOgaStringArray(OgaStringArray* string_array, const char* str) {
	if (!g_initialized || !g_api.StringArrayAddString) return NULL;
	return g_api.StringArrayAddString(string_array, str);
}

OgaResult* ProcessOgaImagesAndPrompts(const OgaMultiModalProcessor* processor,  const OgaStringArray* prompts, const OgaImages* images, OgaNamedTensors** out) {
	if (!g_initialized || !g_api.ProcessorProcessImagesAndPrompts) return NULL;
	return g_api.ProcessorProcessImagesAndPrompts(processor, prompts, images, out);
}

// Engine API initialization
int SetGenAiEngineApi(void* createEngine, void* destroyEngine,
	void* engineStep, void* engineHasPendingRequests,
	void* engineAddRequest, void* engineRemoveRequest,
	void* createRequest, void* destroyRequest,
	void* requestAddTokens, void* requestSetOpaqueData,
	void* requestGetOpaqueData, void* requestHasUnseenTokens,
	void* requestGetUnseenToken, void* requestIsDone) {
	if (g_engine_initialized) return 0;
	if (!createEngine || !destroyEngine || !engineStep || !engineHasPendingRequests ||
		!engineAddRequest || !engineRemoveRequest || !createRequest || !destroyRequest ||
		!requestAddTokens || !requestSetOpaqueData || !requestGetOpaqueData ||
		!requestHasUnseenTokens || !requestGetUnseenToken || !requestIsDone) {
		return 1;
	}
	g_api.CreateEngine = (PFN_OgaCreateEngine) createEngine;
	g_api.DestroyEngine = (PFN_OgaDestroyEngine) destroyEngine;
	g_api.EngineStep = (PFN_OgaEngineStep) engineStep;
	g_api.EngineHasPendingRequests = (PFN_OgaEngineHasPendingRequests) engineHasPendingRequests;
	g_api.EngineAddRequest = (PFN_OgaEngineAddRequest) engineAddRequest;
	g_api.EngineRemoveRequest = (PFN_OgaEngineRemoveRequest) engineRemoveRequest;
	g_api.CreateRequest = (PFN_OgaCreateRequest) createRequest;
	g_api.DestroyRequest = (PFN_OgaDestroyRequest) destroyRequest;
	g_api.RequestAddTokens = (PFN_OgaRequestAddTokens) requestAddTokens;
	g_api.RequestSetOpaqueData = (PFN_OgaRequestSetOpaqueData) requestSetOpaqueData;
	g_api.RequestGetOpaqueData = (PFN_OgaRequestGetOpaqueData) requestGetOpaqueData;
	g_api.RequestHasUnseenTokens = (PFN_OgaRequestHasUnseenTokens) requestHasUnseenTokens;
	g_api.RequestGetUnseenToken = (PFN_OgaRequestGetUnseenToken) requestGetUnseenToken;
	g_api.RequestIsDone = (PFN_OgaRequestIsDone) requestIsDone;
	g_engine_initialized = 1;
	return 0;
}

int GenAiEngineApiIsInitialized(void) { return g_engine_initialized; }

// Engine thin wrappers
OgaResult* CreateOgaEngine(OgaModel* model, OgaEngine** out) {
	if (!g_engine_initialized || !g_api.CreateEngine) return NULL;
	return g_api.CreateEngine(model, out);
}

void DestroyOgaEngine(OgaEngine* engine) {
	if (!engine) return;
	if (!g_engine_initialized || !g_api.DestroyEngine) return;
	g_api.DestroyEngine(engine);
}

OgaResult* EngineStep(OgaEngine* engine, OgaRequest** request) {
	if (!g_engine_initialized || !g_api.EngineStep) return NULL;
	return g_api.EngineStep(engine, request);
}

OgaResult* EngineHasPendingRequests(OgaEngine* engine, bool* out) {
	if (!g_engine_initialized || !g_api.EngineHasPendingRequests) return NULL;
	return g_api.EngineHasPendingRequests(engine, out);
}

OgaResult* EngineAddRequest(OgaEngine* engine, OgaRequest* request) {
	if (!g_engine_initialized || !g_api.EngineAddRequest) return NULL;
	return g_api.EngineAddRequest(engine, request);
}

OgaResult* EngineRemoveRequest(OgaEngine* engine, OgaRequest* request) {
	if (!g_engine_initialized || !g_api.EngineRemoveRequest) return NULL;
	return g_api.EngineRemoveRequest(engine, request);
}

OgaResult* CreateOgaRequest(OgaGeneratorParams* params, OgaRequest** out) {
	if (!g_engine_initialized || !g_api.CreateRequest) return NULL;
	return g_api.CreateRequest(params, out);
}

void DestroyOgaRequest(OgaRequest* request) {
	if (!request) return;
	if (!g_engine_initialized || !g_api.DestroyRequest) return;
	g_api.DestroyRequest(request);
}

OgaResult* RequestAddTokens(OgaRequest* request, const OgaSequences* tokens) {
	if (!g_engine_initialized || !g_api.RequestAddTokens) return NULL;
	return g_api.RequestAddTokens(request, tokens);
}

OgaResult* RequestSetOpaqueData(OgaRequest* request, void* opaque_data) {
	if (!g_engine_initialized || !g_api.RequestSetOpaqueData) return NULL;
	return g_api.RequestSetOpaqueData(request, opaque_data);
}

OgaResult* RequestGetOpaqueData(OgaRequest* request, void** opaque_data) {
	if (!g_engine_initialized || !g_api.RequestGetOpaqueData) return NULL;
	return g_api.RequestGetOpaqueData(request, opaque_data);
}

OgaResult* RequestHasUnseenTokens(const OgaRequest* request, bool* out) {
	if (!g_engine_initialized || !g_api.RequestHasUnseenTokens) return NULL;
	return g_api.RequestHasUnseenTokens(request, out);
}

OgaResult* RequestGetUnseenToken(OgaRequest* request, int32_t* out) {
	if (!g_engine_initialized || !g_api.RequestGetUnseenToken) return NULL;
	return g_api.RequestGetUnseenToken(request, out);
}

OgaResult* RequestIsDone(const OgaRequest* request, bool* out) {
	if (!g_engine_initialized || !g_api.RequestIsDone) return NULL;
	return g_api.RequestIsDone(request, out);
}