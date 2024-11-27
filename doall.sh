#!/bin/bash

# Ścieżka do katalogu głównego projektu
PROJECT_DIR=$(pwd)

# Plik, do którego będą zapisywane wszystkie zawartości
OUTPUT_FILE="$PROJECT_DIR/all.txt"

# Tworzenie pustego pliku wyjściowego lub wyczyszczenie istniejącego
> "$OUTPUT_FILE"

# Funkcja przetwarzająca pliki w katalogu
process_files() {
    local dir_path="$1"
    for file in "$dir_path"/*; do
        # Pomiń plik wyjściowy
        if [ "$file" == "$OUTPUT_FILE" ]; then
            continue
        fi

        # Pomiń pliki z rozszerzeniem .log
        if [[ "$file" == *.log ]]; then
            echo "Pomijanie pliku log: $file"
            continue
        fi

        if [[ "$file" == *.gob ]]; then
            echo "Pomijanie pliku gob: $file"
            continue
        fi
        if [[ "$file" == README ]]; then
            echo "Pomijanie pliku gob: $file"
            continue
        fi
        if [[ "$file" == LICENSE ]]; then
            echo "Pomijanie pliku gob: $file"
            continue
        fi
        if [ -f "$file" ]; then
            echo "# Plik $file" >> "$OUTPUT_FILE"
            cat "$file" >> "$OUTPUT_FILE"
            echo -e "\n# Koniec $file\n" >> "$OUTPUT_FILE"
        elif [ -d "$file" ]; then
            process_files "$file"
        fi
    done
}

# Przetwarzanie wszystkich plików w katalogu głównym projektu
process_files "$PROJECT_DIR"

echo "Zawartość wszystkich plików (bez plików .log) została zapisana do $OUTPUT_FILE"
