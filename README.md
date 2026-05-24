# vmxtool - tool to edit VMware VMX files

This is a clone of the VMware dictTool command first released in
Fusion and Workstation 25H2. This tool was created so that users of 
earlier versions of VMware products can also make use of this useful tool.

The tool implements the same features as the VMware tool and ensures that
the format of the VMX file is not altered during editing, by preserving
white space and comments. It can also be used on other VMware dictionary
files such as config and preferences.

Version 2 adds a lot of new functions to manipulate VMX based files such as
commenting lines, sorting, grouping and templates.

```
A tool to examine and modify VMware VMX configuration files.

BASIC COMMANDS:
    help                    Print this help message
    version                 Print version information
    print FILE              Print the contents of FILE
    query FILE KEY          Print the value for KEY
    add FILE KEY=VALUE      Add a new entry (fails if exists)
    set FILE KEY=VALUE      Set an entry (adds or updates)
    remove FILE KEY         Remove an entry

PREFIX OPERATIONS:
    add-prefix FILE KEY PREFIX
        Add PREFIX to the beginning of KEY

    remove-prefix FILE KEY PREFIX
        Remove PREFIX from the beginning of KEY

    remove-prefix-all FILE PREFIX
        Remove all entries with PREFIX

INSERTION COMMANDS:
    insert-comment-before FILE KEY COMMENT
        Insert a comment line before KEY

    insert-comment-after FILE KEY COMMENT
        Insert a comment line after KEY

    insert-blank-before FILE KEY [COUNT]
        Insert blank line(s) before KEY (default: 1)

    insert-blank-after FILE KEY [COUNT]
        Insert blank line(s) after KEY (default: 1)

REMOVAL COMMANDS:
    remove-comment-before FILE KEY
        Remove comment line immediately before KEY

    remove-comment-after FILE KEY
        Remove comment line immediately after KEY

    remove-blank-before FILE KEY [COUNT]
        Remove blank line(s) immediately before KEY (default: 1)

    remove-blank-after FILE KEY [COUNT]
        Remove blank line(s) immediately after KEY (default: 1)

    remove-all-comments FILE
        Remove all comment lines from the file

    remove-all-blanks FILE
        Remove all blank lines from the file

ORGANIZATION:
    sort FILE [--preserve-sections]
        Sort entries alphabetically

    group FILE PREFIX [--after KEY]
        Group all entries with PREFIX together

    add-section FILE KEY "SECTION NAME"
        Add a formatted section header before KEY

SEARCH & QUERY:
    list-keys FILE [--prefix PREFIX]
        List all keys, optionally filtered by prefix

    search FILE PATTERN [--values]
        Search for keys (or values with --values) matching PATTERN

    diff FILE1 FILE2
        Show differences between two files

    validate FILE
        Check for common issues (duplicates, empty values, etc.)

BULK OPERATIONS:
    rename FILE PATTERN REPLACEMENT
        Rename keys using regex (e.g., "serial(\d+)" "uart\1")

    comment FILE KEY
        Comment out an entry (disable without deleting)

    uncomment FILE KEY
        Uncomment a previously commented entry

    bulk-set FILE --from SOURCE
        Set multiple entries from SOURCE file or stdin (-)

IMPORT/EXPORT:
    export FILE --format FORMAT
        Export to json or yaml

    import FILE SOURCE [--format FORMAT]
        Import from json or yaml file

    extract FILE PATTERN --output OUTPUT
        Extract entries matching PATTERN to OUTPUT

    merge FILE1 FILE2 [--output OUTPUT] [--overwrite]
        Merge FILE2 into FILE1

    apply-template FILE TEMPLATE --var NAME=VALUE ...
        Apply TEMPLATE with variable substitution

OPTIONS:
    --backup                Create backup before modifying (for set/add/remove)
```
(c) 2025-2026 David Parsons