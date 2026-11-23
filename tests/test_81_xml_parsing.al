// Copyright 2026 Pawl Programming Language
// Test 81: XML parsing - XML structure handling.

// 1. Simple XML tag parsing simulation
func parse_xml_tag(xml: string) -> string {
    start := string.index_of(xml, "<")
    end := string.index_of(xml, ">")
    if start >= 0 and end > start {
        return string.substring(xml, start + 1, end)
    }
    return ""
}
tag := parse_xml_tag("<root>content</root>")
assert tag == "root"

// 2. XML attribute parsing
func parse_attribute(xml: string, attr: string) -> string {
    attr_pattern := attr + "=\""
    start := string.index_of(xml, attr_pattern)
    if start >= 0 {
        start = start + len(attr_pattern)
        end := string.index_of(xml, "\"", start)
        if end > start {
            return string.substring(xml, start, end)
        }
    }
    return ""
}
xml_attr := "<element id=\"123\" name=\"test\"/>"
id_val := parse_attribute(xml_attr, "id")
assert id_val == "123"

// 3. XML content extraction
func extract_content(xml: string) -> string {
    start := string.index_of(xml, ">") + 1
    end := string.index_of(xml, "<", start)
    if start > 0 and end > start {
        return string.substring(xml, start, end)
    }
    return ""
}
content := extract_content("<tag>content</tag>")
assert content == "content"

// 4. Nested XML parsing
func extract_nested(xml: string, tag: string) -> string {
    open_tag := "<" + tag + ">"
    close_tag := "</" + tag + ">"
    start := string.index_of(xml, open_tag)
    if start >= 0 {
        start = start + len(open_tag)
        end := string.index_of(xml, close_tag, start)
        if end > start {
            return string.substring(xml, start, end)
        }
    }
    return ""
}
nested := extract_nested("<outer><inner>value</inner></outer>", "inner")
assert nested == "value"

// 5. XML validation
func is_valid_xml(xml: string) -> bool {
    has_open := string.contains(xml, "<")
    has_close := string.contains(xml, ">")
    return has_open and has_close
}
assert is_valid_xml("<root></root>")
assert !is_valid_xml("not xml")

// 6. XML self-closing tag
func is_self_closing(xml: string) -> bool {
    return string.contains(xml, "/>")
}
assert is_self_closing("<tag/>")
assert !is_self_closing("<tag></tag>")

// 7. XML comment parsing
func parse_comment(xml: string) -> string {
    start := string.index_of(xml, "<!--")
    end := string.index_of(xml, "-->")
    if start >= 0 and end > start {
        return string.substring(xml, start + 4, end)
    }
    return ""
}
comment := parse_comment("<!-- comment -->")
assert comment == " comment "

// 8. CDATA section
func parse_cdata(xml: string) -> string {
    start := string.index_of(xml, "<![CDATA[")
    end := string.index_of(xml, "]]>")
    if start >= 0 and end > start {
        return string.substring(xml, start + 9, end)
    }
    return ""
}
cdata := parse_cdata("<![CDATA[data]]>")
assert cdata == "data"

// 9. XML declaration
func parse_declaration(xml: string) -> string {
    start := string.index_of(xml, "<?xml")
    end := string.index_of(xml, "?>")
    if start >= 0 and end > start {
        return string.substring(xml, start, end + 2)
    }
    return ""
}
decl := parse_declaration("<?xml version=\"1.0\"?>")
assert string.contains(decl, "xml")

// 10. XML namespace
func parse_namespace(xml: string) -> string {
    ns_pattern := "xmlns=\""
    start := string.index_of(xml, ns_pattern)
    if start >= 0 {
        start = start + len(ns_pattern)
        end := string.index_of(xml, "\"", start)
        if end > start {
            return string.substring(xml, start, end)
        }
    }
    return ""
}
ns := parse_namespace("<root xmlns=\"http://example.com\"/>")
assert ns == "http://example.com"

println("✓ test_81_xml_parsing passed")
