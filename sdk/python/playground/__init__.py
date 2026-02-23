from .bot import Bot
from .router import BotRouter

# Deprecated alias for backward compatibility
Agent = Bot
from .types import (
    AIConfig,
    CompactDiscoveryResponse,
    DiscoveryResponse,
    DiscoveryResult,
    MemoryConfig,
    BotDefinition,
    SkillDefinition,
)
from .multimodal import (
    Text,
    Image,
    Audio,
    File,
    MultimodalContent,
    text,
    image_from_file,
    image_from_url,
    audio_from_file,
    audio_from_url,
    file_from_path,
    file_from_url,
)
from .multimodal_response import (
    MultimodalResponse,
    AudioOutput,
    ImageOutput,
    FileOutput,
    detect_multimodal_response,
)
from .media_providers import (
    MediaProvider,
    FalProvider,
    LiteLLMProvider,
    OpenRouterProvider,
    get_provider,
    register_provider,
)

__all__ = [
    "Bot",
    "Agent",  # Deprecated alias
    "AIConfig",
    "MemoryConfig",
    "BotDefinition",
    "SkillDefinition",
    "DiscoveryResponse",
    "CompactDiscoveryResponse",
    "DiscoveryResult",
    "BotRouter",
    # Input multimodal classes
    "Text",
    "Image",
    "Audio",
    "File",
    "MultimodalContent",
    # Convenience functions for input
    "text",
    "image_from_file",
    "image_from_url",
    "audio_from_file",
    "audio_from_url",
    "file_from_path",
    "file_from_url",
    # Output multimodal classes
    "MultimodalResponse",
    "AudioOutput",
    "ImageOutput",
    "FileOutput",
    "detect_multimodal_response",
    # Media providers
    "MediaProvider",
    "FalProvider",
    "LiteLLMProvider",
    "OpenRouterProvider",
    "get_provider",
    "register_provider",
]

__version__ = "0.1.41-rc.75"
