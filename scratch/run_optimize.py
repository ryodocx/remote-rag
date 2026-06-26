import sys
from src.database.client import DatabaseClient

def main():
    sys.stdout.reconfigure(encoding='utf-8')
    client = DatabaseClient()
    print("Running optimization on the production LanceDB table...")
    client.optimize()
    print("Optimization and FTS index rebuild completed successfully.")

if __name__ == "__main__":
    main()
