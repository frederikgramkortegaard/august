// Text Processing Contract - showcases string operations, arrays, and maps together
define analyze_text(text: string) : map[string]int {
    stats: map[string]int = {}

    // Basic text statistics
    text_length: int = len(text)
    stats["length"] = text_length

    // Count characters (simplified - just count specific ones)
    char_counts: map[string]int = {}
    char_counts["a"] = 0
    char_counts["e"] = 0
    char_counts["i"] = 0
    char_counts["o"] = 0
    char_counts["u"] = 0

    // Iterate through each character
    i: int = 0
    while i < text_length {
        char: string = text[i]

        // Check for vowels
        if char == "a" {
            char_counts["a"] = char_counts["a"] + 1
        }
        if char == "e" {
            char_counts["e"] = char_counts["e"] + 1
        }
        if char == "i" {
            char_counts["i"] = char_counts["i"] + 1
        }
        if char == "o" {
            char_counts["o"] = char_counts["o"] + 1
        }
        if char == "u" {
            char_counts["u"] = char_counts["u"] + 1
        }

        i = i + 1
    }

    // Store vowel counts in main stats
    stats["vowel_a"] = char_counts["a"]
    stats["vowel_e"] = char_counts["e"]
    stats["vowel_i"] = char_counts["i"]
    stats["vowel_o"] = char_counts["o"]
    stats["vowel_u"] = char_counts["u"]

    // Total vowels
    total_vowels: int = char_counts["a"] + char_counts["e"] + char_counts["i"] + char_counts["o"] + char_counts["u"]
    stats["total_vowels"] = total_vowels

    return stats
}

define build_word_frequency(words: []string) : map[string]int {
    frequency: map[string]int = {}

    i: int = 0
    while i < len(words) {
        word: string = words[i]

        // Initialize count if not exists (simplified)
        // In real implementation we'd check if key exists first
        frequency[word] = 1  // Just set to 1 for demo

        i = i + 1
    }

    return frequency
}

define find_longest_word(words: []string) : string {
    if len(words) == 0 {
        return ""
    }

    longest: string = words[0]
    max_length: int = len(longest)

    i: int = 1
    while i < len(words) {
        current_word: string = words[i]
        current_length: int = len(current_word)

        if current_length > max_length {
            longest = current_word
            max_length = current_length
        }

        i = i + 1
    }

    return longest
}

define create_text_summary(text: string, words: []string) : map[string]string {
    summary: map[string]string = {}

    // Basic info
    length_str: string = "length"  // Would convert int to string in real system
    summary["text_length"] = length_str

    // Find longest word
    longest: string = find_longest_word(words)
    summary["longest_word"] = longest

    // Simple text preview (first 20 chars)
    preview: string = ""
    text_len: int = len(text)
    preview_len: int = 20

    if text_len > preview_len {
        // Would do proper substring in real system
        summary["preview"] = "preview..."
    } else {
        summary["preview"] = text
    }

    return summary
}

define process_document(title: string, content: string, tags: []string) : map[string]map[string]int {
    document_data: map[string]map[string]int = {}

    // Analyze the title
    title_stats: map[string]int = analyze_text(title)
    // Can't directly assign map to map[string]map[string]int yet
    // This would work in a full implementation

    // Analyze the content
    content_stats: map[string]int = analyze_text(content)

    // For demo, just return empty structure
    return document_data
}