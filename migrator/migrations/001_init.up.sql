CREATE TABLE rooms (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    capacity INT NOT NULL CHECK (capacity > 0)
);

CREATE TABLE messages (
    id BIGSERIAL PRIMARY KEY,
    room_id VARCHAR(50) NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    sender_name VARCHAR(100) NOT NULL,
    sender_ip VARCHAR(45) NOT NULL,
    content TEXT NOT NULL
);