"""
Database models and initialization for JPEG to HEIC converter
"""
import os
import logging
from datetime import datetime
from typing import Optional
from sqlalchemy import create_engine, Column, Integer, String, Float, DateTime, Boolean, Text
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker, Session

logger = logging.getLogger(__name__)

Base = declarative_base()


class Task(Base):
    """Task model for tracking conversion jobs"""
    __tablename__ = "tasks"

    id = Column(Integer, primary_key=True, autoincrement=True)
    task_type = Column(String(10), nullable=False)  # 'once' or 'watch'
    source_path = Column(String(512), nullable=False)
    target_path = Column(String(512), nullable=True)
    status = Column(String(20), nullable=False, default='pending')  # pending, running, success, failed
    error_message = Column(Text, nullable=True)
    start_time = Column(DateTime, nullable=True)
    end_time = Column(DateTime, nullable=True)
    duration = Column(Float, nullable=True)  # in seconds
    
    # Metadata preservation tracking
    metadata_preserved = Column(Boolean, default=False)
    metadata_summary = Column(Text, nullable=True)  # JSON or text summary of preserved fields
    source_datetime = Column(String(50), nullable=True)  # DateTimeOriginal from source
    target_datetime = Column(String(50), nullable=True)  # DateTimeOriginal in target
    
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    def to_dict(self):
        """Convert task to dictionary"""
        return {
            'id': self.id,
            'task_type': self.task_type,
            'source_path': self.source_path,
            'target_path': self.target_path,
            'status': self.status,
            'error_message': self.error_message,
            'start_time': self.start_time.isoformat() if self.start_time else None,
            'end_time': self.end_time.isoformat() if self.end_time else None,
            'duration': self.duration,
            'metadata_preserved': self.metadata_preserved,
            'metadata_summary': self.metadata_summary,
            'source_datetime': self.source_datetime,
            'target_datetime': self.target_datetime,
            'datetime_consistent': self.source_datetime == self.target_datetime if self.source_datetime and self.target_datetime else None,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None,
        }


class Database:
    """Database manager"""
    
    def __init__(self, db_path: str):
        self.db_path = db_path
        self.engine = None
        self.SessionLocal = None
        self._initialize()
    
    def _initialize(self):
        """Initialize database connection and create tables"""
        # Ensure directory exists
        db_dir = os.path.dirname(self.db_path)
        if db_dir and not os.path.exists(db_dir):
            os.makedirs(db_dir, exist_ok=True)
        
        # Create engine
        self.engine = create_engine(
            f'sqlite:///{self.db_path}',
            connect_args={"check_same_thread": False},
            echo=False
        )
        
        # Create tables
        Base.metadata.create_all(bind=self.engine)
        
        # Create session factory
        self.SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=self.engine)
        
        logger.info(f"Database initialized at {self.db_path}")
    
    def get_session(self) -> Session:
        """Get a new database session"""
        return self.SessionLocal()
    
    def create_task(self, task_type: str, source_path: str) -> Task:
        """Create a new task"""
        session = self.get_session()
        try:
            task = Task(
                task_type=task_type,
                source_path=source_path,
                status='pending'
            )
            session.add(task)
            session.commit()
            session.refresh(task)
            return task
        finally:
            session.close()
    
    def update_task(self, task_id: int, **kwargs):
        """Update task fields"""
        session = self.get_session()
        try:
            task = session.query(Task).filter(Task.id == task_id).first()
            if task:
                for key, value in kwargs.items():
                    setattr(task, key, value)
                session.commit()
        finally:
            session.close()
    
    def get_task(self, task_id: int) -> Optional[Task]:
        """Get task by ID"""
        session = self.get_session()
        try:
            return session.query(Task).filter(Task.id == task_id).first()
        finally:
            session.close()
    
    def get_tasks(self, task_type: Optional[str] = None, status: Optional[str] = None, 
                  limit: int = 100, offset: int = 0):
        """Get tasks with optional filters"""
        session = self.get_session()
        try:
            query = session.query(Task)
            
            if task_type:
                query = query.filter(Task.task_type == task_type)
            if status:
                query = query.filter(Task.status == status)
            
            query = query.order_by(Task.created_at.desc())
            query = query.limit(limit).offset(offset)
            
            return query.all()
        finally:
            session.close()
    
    def get_stats(self):
        """Get conversion statistics"""
        session = self.get_session()
        try:
            total = session.query(Task).count()
            success = session.query(Task).filter(Task.status == 'success').count()
            failed = session.query(Task).filter(Task.status == 'failed').count()
            running = session.query(Task).filter(Task.status == 'running').count()
            pending = session.query(Task).filter(Task.status == 'pending').count()
            
            # Metadata preservation stats
            metadata_preserved = session.query(Task).filter(
                Task.status == 'success',
                Task.metadata_preserved == True
            ).count()
            
            metadata_rate = (metadata_preserved / success * 100) if success > 0 else 0
            
            return {
                'total': total,
                'success': success,
                'failed': failed,
                'running': running,
                'pending': pending,
                'metadata_preserved': metadata_preserved,
                'metadata_preservation_rate': round(metadata_rate, 2)
            }
        finally:
            session.close()
