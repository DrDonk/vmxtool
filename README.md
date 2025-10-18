# vmxtool - tool to edit VMware VMX files

This is a clone of the VMware dictTool command first released in
Fusion and Workstation 25H2. This tool was created so that users of 
earlier versions of VMware products can also make use of this useful tool.

The tool implements the same features as the VMware tool and ensures that
the format of the VMX file is not altered during editing, by preserving
white space and comments. It can also be used on other VMware dictionary
files such as config and preferences.

```
A tool to examine and modify VMware VMX configuration files.

Available commands:
    help
        Prints help.

    version
        Prints version information.

    print FILE
        Prints the contents of the specified VMX file.

    add FILE KEY=VALUE
        Adds a new entry to the specified VMX file.
        Fails if the key already exists.

    set FILE KEY=VALUE
        Sets an entry in the specified VMX file, adding it if it does
        not already exist.

    remove FILE KEY
        Removes the entry with the specified key from the specified VMX
        file. Fails if the key does not exist.

    query FILE KEY
        Prints the value for the specified key from the specified VMX
        file. Fails if the key does not exist.
```
(c) 2025 David Parsons